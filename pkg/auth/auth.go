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

func NewTransport(insecure bool) http.RoundTripper {
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

func OauthHttpClient(token *config.TargetToken, insecure bool) *http.Client {
	var oAuthToken *oauth2.Token
	if token != nil {
		oAuthToken = &oauth2.Token{
			TokenType:   token.Type,
			AccessToken: token.Value,
		}
	}

	transport := NewTransport(insecure)

	if token != nil {
		transport = &oauth2.Transport{
			Source: oauth2.StaticTokenSource(oAuthToken),
			Base:   transport,
		}
	}

	return &http.Client{Transport: transport}
}

func PasswordGrant(httpClient *http.Client, url, username, password string) (string, string, error) {

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
