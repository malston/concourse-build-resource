package main

import (
	"net/http"

	"github.com/jchesterpivotal/concourse-build-resource/pkg/auth"
	"github.com/jchesterpivotal/concourse-build-resource/pkg/check"
	"github.com/jchesterpivotal/concourse-build-resource/pkg/config"

	"encoding/json"
	"log"
	"os"
)

func main() {
	var request config.CheckRequest
	err := json.NewDecoder(os.Stdin).Decode(&request)
	if err != nil {
		log.Fatalf("failed to parse input JSON: %s", err)
	}

	var tokenType, tokenValue string
	if request.Source.Username != "" && request.Source.Password != "" {
		httpClient := &http.Client{Transport: auth.NewTransport(true)}
		tokenType, tokenValue, err = auth.PasswordGrant(httpClient, request.Source.ConcourseUrl, request.Source.Username, request.Source.Password)
		if err != nil {
			log.Fatalf("failed to perform 'check': %s", err)
		}
	}

	checkResponse, err := check.NewChecker(&request, tokenType, tokenValue).Check()
	if err != nil {
		log.Fatalf("failed to perform 'check': %s", err)
	}

	err = json.NewEncoder(os.Stdout).Encode(checkResponse)
	if err != nil {
		log.Fatalf("failed to encode check.Check response: %s", err.Error())
	}
}
