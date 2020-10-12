package auth

import (
	"context"
	"net"

	"crypto/tls"
	"net/http"
	"time"

	"github.com/jchesterpivotal/concourse-build-resource/pkg/config"
	"golang.org/x/oauth2"
)

type basicAuthTransport struct {
	username string
	password string
	token    string

	base http.RoundTripper
}

func (t basicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.SetBasicAuth(t.username, t.password)
	return t.base.RoundTrip(r)
}

func transport(insecure bool) http.RoundTripper {
	var transport http.RoundTripper

	transport = &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
			RootCAs:            nil,
			Certificates:       []tls.Certificate{},
		},
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
		Proxy: http.ProxyFromEnvironment,
	}

	return transport
}

func BasicAuthHttpClient(username string, password string, insecure bool) *http.Client {
	return &http.Client{
		Transport: basicAuthTransport{
			username: username,
			password: password,
			token:    password,
			base:     transport(insecure),
		},
	}
}

func OauthHttpClient(token *config.TargetToken, insecure bool) *http.Client {
	var oAuthToken *oauth2.Token
	if token != nil {
		oAuthToken = &oauth2.Token{
			TokenType:   token.Type,
			AccessToken: token.Value,
		}
	}

	base := transport(insecure)

	if token != nil {
		transport := &oauth2.Transport{
			Source: oauth2.StaticTokenSource(oAuthToken),
			Base:   base,
		}
		return &http.Client{Transport: transport}
	}

	return &http.Client{Transport: base}
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

func Login(username, password, concourseURL string) (string, string, error) {
	httpClient := &http.Client{Transport: transport(true)}
	return passwordGrant(httpClient, concourseURL, username, password)
}
