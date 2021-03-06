package config

type Source struct {
	ConcourseUrl   string `json:"concourse_url"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	Team           string `json:"team"`
	Pipeline       string `json:"pipeline"`
	Job            string `json:"job,omitempty"`
	InitialBuildId int    `json:"initial_build_id,omitempty"`
	FetchPageSize  int    `json:"fetch_page_size,omitempty"`
	EnableTracing  bool   `json:"enable_tracing,omitempty"`
}

type Version struct {
	BuildId string `json:"build_id"`
}

type InParams struct{}

type VersionMetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type TargetToken struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type InRequest struct {
	Source           Source   `json:"source"`
	Version          Version  `json:"version"`
	Params           InParams `json:"params,omitempty"`
	WorkingDirectory string   `json:"working_directory,omitempty"`
	ReleaseVersion   string
	ReleaseGitRef    string
	GetTimestamp     int64
	GetUuid          string
	Token            *TargetToken `json:"token,omitempty"`
}

type InResponse struct {
	Version  Version                `json:"version"`
	Metadata []VersionMetadataField `json:"metadata"`
}

type CheckRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version,omitempty"`
}

type CheckResponse []Version
