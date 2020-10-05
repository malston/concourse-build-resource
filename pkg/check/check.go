package check

import (
	"context"
	"log"
	"net"
	"strings"

	"github.com/jchesterpivotal/concourse-build-resource/pkg/config"
	"golang.org/x/oauth2"

	"github.com/concourse/concourse/atc"
	gc "github.com/concourse/concourse/go-concourse/concourse"

	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Checker interface {
	Check() (*config.CheckResponse, error)
}

type checker struct {
	checkRequest    *config.CheckRequest
	concourseClient gc.Client
	concourseTeam   gc.Team
}

const defaultVersionPageSize = 100

func (c checker) Check() (*config.CheckResponse, error) {
	if c.checkRequest.Source.EnableTracing {
		log.Printf("Received CheckRequest: %+v", c.checkRequest)
	}

	version := c.checkRequest.Version
	initialBuildId := c.checkRequest.Source.InitialBuildId
	versionPageSize := c.checkRequest.Source.FetchPageSize
	if versionPageSize == 0 {
		versionPageSize = defaultVersionPageSize
	}

	if version.BuildId == "" && initialBuildId > 0 {
		return &config.CheckResponse{{BuildId: strconv.Itoa(initialBuildId)}}, nil
	}

	if version.BuildId == "" {
		builds, err := c.getBuilds(gc.Page{Limit: 1})
		if err != nil {
			return nil, err
		}

		buildId := strconv.Itoa(builds[0].ID)
		return &config.CheckResponse{
			config.Version{BuildId: buildId},
		}, nil
	}

	buildId, err := strconv.Atoi(version.BuildId)
	if err != nil {
		return nil, fmt.Errorf("could not convert build id '%s' to an int: '%s", version.BuildId, err.Error())
	}

	builds, err := c.getBuilds(gc.Page{Since: buildId, Limit: versionPageSize})
	if err != nil {
		return nil, err
	}

	if len(builds) == 0 { // there are no builds at all
		return &config.CheckResponse{}, nil
	}

	newBuilds := make(config.CheckResponse, 0)
	for _, b := range builds {
		if b.Status != string(atc.StatusStarted) && b.Status != string(atc.StatusPending) {
			newBuildId := strconv.Itoa(b.ID)
			newBuilds = append(newBuilds, config.Version{BuildId: newBuildId})
		}
	}

	if len(newBuilds) == 0 { // there were no new builds
		return &config.CheckResponse{version}, nil
	}

	return &newBuilds, nil
}

// BasicAuthRoundTripper implements the http.RoundTripper interface
type BasicAuthRoundTripper struct {
	Proxied  http.RoundTripper
	Username string
	Password string
}

// RoundTrip sets username and password for a basic auth request
func (brt BasicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if brt.Proxied == nil {
		brt.Proxied = http.DefaultTransport
	}

	if len(brt.Username) > 0 && len(brt.Password) > 0 {
		rClone := new(http.Request)
		*rClone = *req
		rClone.Header = make(http.Header, len(req.Header))
		for idx, header := range req.Header {
			rClone.Header[idx] = append([]string(nil), header...)
		}
		rClone.SetBasicAuth(brt.Username, brt.Password)
		return brt.Proxied.RoundTrip(rClone)
	}
	return brt.Proxied.RoundTrip(req)
}

func defaultHttpClient(token *config.TargetToken, insecure bool) *http.Client {
	var oAuthToken *oauth2.Token
	if token != nil {
		oAuthToken = &oauth2.Token{
			TokenType:   token.Type,
			AccessToken: token.Value,
		}
	}

	transport := transport(insecure)

	if token != nil {
		transport = &oauth2.Transport{
			Source: oauth2.StaticTokenSource(oAuthToken),
			Base:   transport,
		}
	}

	return &http.Client{Transport: transport}
}

func passwordGrant(httpClient *http.Client, url, username, password string) (string, string, error) {

	oauth2Config := oauth2.Config{
		ClientID:     "fly",
		ClientSecret: "Zmx5",
		Endpoint:     oauth2.Endpoint{TokenURL: url + "/sky/issuer/token"},
		Scopes:       []string{"openid", "profile", "email", "federated:id", "groups"},
	}

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	token, err := oauth2Config.PasswordCredentialsToken(ctx, username, password)
	if err != nil {
		return "", "", err
	}

	return token.TokenType, token.AccessToken, nil
}

func transport(insecure bool) http.RoundTripper {
	var transport http.RoundTripper

	transport = &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
			// RootCAs:            caCertPool,
			// Certificates:       clientCertificate,
		},
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
		Proxy: http.ProxyFromEnvironment,
	}

	return transport
}

func NewChecker(input *config.CheckRequest) Checker {
	var tokenType, tokenValue string
	var err error
	httpClient := defaultHttpClient(nil, true)
	if len(input.Source.Username) > 0 && len(input.Source.Password) > 0 {
		tokenType, tokenValue, err = passwordGrant(httpClient, input.Source.ConcourseUrl, input.Source.Username, input.Source.Password)
		if err != nil {
			httpClient = &http.Client{Transport: BasicAuthRoundTripper{transport(true), input.Source.Username, input.Source.Password}}
		} else {
			httpClient = defaultHttpClient(&config.TargetToken{Type: tokenType, Value: tokenValue}, true)
		}
	}
	concourse := gc.NewClient(input.Source.ConcourseUrl, httpClient, input.Source.EnableTracing)

	return NewCheckerUsingClient(input, concourse)
}

func NewCheckerUsingClient(input *config.CheckRequest, client gc.Client) Checker {
	input.Source.ConcourseUrl = strings.TrimSuffix(input.Source.ConcourseUrl, "/")

	return checker{
		checkRequest:    input,
		concourseClient: client,
		concourseTeam:   client.Team(input.Source.Team),
	}
}

func (c checker) getBuilds(initialPage gc.Page) ([]atc.Build, error) {
	concourseUrl := c.checkRequest.Source.ConcourseUrl
	team := c.checkRequest.Source.Team
	pipeline := c.checkRequest.Source.Pipeline
	job := c.checkRequest.Source.Job
	builds := make([]atc.Build, 0)
	pageBuilds := make([]atc.Build, 0)
	var pagination gc.Pagination
	var found bool
	var err error

	// latest version only case
	if initialPage.Limit == 1 {
		if job == "" && pipeline == "" && team == "" {
			builds, _, err = c.concourseClient.Builds(initialPage)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve builds for concourse URL '%s': %s", concourseUrl, err.Error())
			}
		} else if job == "" && pipeline == "" {
			builds, _, err = c.concourseTeam.Builds(initialPage)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve builds for team '%s': %s", team, err.Error())
			}
		} else if job == "" {
			builds, _, found, err = c.concourseTeam.PipelineBuilds(pipeline, initialPage)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve builds for pipeline '%s': %s", pipeline, err.Error())
			}
			if !found {
				return nil, fmt.Errorf("server could not find pipeline '%s'", pipeline)
			}
		} else {
			builds, _, found, err = c.concourseTeam.JobBuilds(pipeline, job, initialPage)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve builds for pipeline/job '%s/%s': %s", pipeline, job, err.Error())
			}
			if !found {
				return nil, fmt.Errorf("server could not find pipeline/job '%s/%s'", pipeline, job)
			}
		}
	} else {
		// versions-since or initial_build_id cases
		if job == "" && pipeline == "" && team == "" {
			_, pagination, err = c.concourseClient.Builds(initialPage)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve builds for concourse URL '%s': %s", concourseUrl, err.Error())
			}

			for pagination.Previous != nil {
				pageBuilds, pagination, err = c.concourseClient.Builds(*pagination.Previous)
				if err != nil {
					return nil, fmt.Errorf("could not retrieve builds for concourse URL '%s': %s", concourseUrl, err.Error())
				}

				builds = append(pageBuilds, builds...)
			}
		} else if job == "" && pipeline == "" {
			_, pagination, err = c.concourseTeam.Builds(initialPage)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve builds for team '%s': %s", team, err.Error())
			}

			for pagination.Previous != nil {
				pageBuilds, pagination, err = c.concourseTeam.Builds(*pagination.Previous)
				if err != nil {
					return nil, fmt.Errorf("could not retrieve builds for concourse URL '%s': %s", concourseUrl, err.Error())
				}

				builds = append(pageBuilds, builds...)
			}
		} else if job == "" {
			_, pagination, found, err = c.concourseTeam.PipelineBuilds(pipeline, initialPage)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve builds for pipeline '%s': %s", pipeline, err.Error())
			}
			if !found {
				return nil, fmt.Errorf("server could not find pipeline '%s'", pipeline)
			}

			for pagination.Previous != nil {
				pageBuilds, pagination, found, err = c.concourseTeam.PipelineBuilds(pipeline, *pagination.Previous)
				if err != nil {
					return nil, fmt.Errorf("could not retrieve builds for concourse URL '%s': %s", concourseUrl, err.Error())
				}
				if !found {
					return nil, fmt.Errorf("server could not find pipeline '%s'", pipeline)
				}

				builds = append(pageBuilds, builds...)
			}
		} else {
			_, pagination, found, err = c.concourseTeam.JobBuilds(pipeline, job, initialPage)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve builds for pipeline/job '%s/%s': %s", pipeline, job, err.Error())
			}
			if !found {
				return nil, fmt.Errorf("server could not find pipeline/job '%s/%s'", pipeline, job)
			}

			for pagination.Previous != nil {
				pageBuilds, pagination, found, err = c.concourseTeam.JobBuilds(pipeline, job, *pagination.Previous)
				if err != nil {
					return nil, fmt.Errorf("could not retrieve builds for pipeline/job '%s/%s': %s", pipeline, job, err.Error())
				}
				if !found {
					return nil, fmt.Errorf("server could not find pipeline/job '%s/%s'", pipeline, job)
				}

				builds = append(pageBuilds, builds...)
			}
		}
	}

	return reverseOrder(builds), nil
}

func reverseOrder(builds []atc.Build) []atc.Build {
	for i, j := 0, len(builds)-1; i < j; i, j = i+1, j-1 {
		builds[i], builds[j] = builds[j], builds[i]
	}
	return builds
}
