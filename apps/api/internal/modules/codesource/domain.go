package codesource

import "time"

const (
	TypeGit        = "git"
	TypeLocal      = "local"
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type CodeSource struct {
	ID            string    `json:"id"`
	SystemID      string    `json:"systemId"`
	Name          string    `json:"name"`
	SourceType    string    `json:"sourceType"`
	RepositoryURL string    `json:"repositoryUrl,omitempty"`
	LocalPath     string    `json:"localPath,omitempty"`
	DefaultRef    string    `json:"defaultRef,omitempty"`
	IncludePaths  []string  `json:"includePaths"`
	ExcludePaths  []string  `json:"excludePaths"`
	CredentialRef string    `json:"credentialRef,omitempty"`
	Status        string    `json:"status"`
	CreatedBy     string    `json:"createdBy"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
