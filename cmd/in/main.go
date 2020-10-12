package main

import (
	"time"

	"github.com/jchesterpivotal/concourse-build-resource/pkg/auth"
	"github.com/jchesterpivotal/concourse-build-resource/pkg/config"
	"github.com/jchesterpivotal/concourse-build-resource/pkg/in"
	uuid "github.com/nu7hatch/gouuid"

	"encoding/json"
	"log"
	"os"
)

var (
	releaseVersion = "unknown"
	releaseGitRef  = "unknown"
)

func main() {
	var request config.InRequest
	err := json.NewDecoder(os.Stdin).Decode(&request)
	if err != nil {
		log.Fatalf("failed to parse input JSON: %s", err)
	}

	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatalf("failed to generate UUID: %s", err)
	}
	request.GetUuid = uuid.String()
	request.WorkingDirectory = os.Args[1]
	request.ReleaseVersion = releaseVersion
	request.ReleaseGitRef = releaseGitRef
	request.GetTimestamp = time.Now().UTC().Unix()

	var tokenType, tokenValue string
	if request.Source.Username != "" && request.Source.Password != "" {
		tokenType, tokenValue, err = auth.Login(request.Source.ConcourseUrl, request.Source.Username, request.Source.Password)
		if err != nil {
			log.Fatalf("failed to perform 'in': %s", err)
		}
	}

	inResponse, err := in.NewInner(&request, tokenType, tokenValue).In()
	if err != nil {
		log.Fatalf("failed to perform 'in': %s", err)
	}

	json.NewEncoder(os.Stdout).Encode(inResponse)
	if err != nil {
		log.Fatalf("failed to encode in.In response: %s", err.Error())
	}
}
