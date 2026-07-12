package environment

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

var (
	ErrEnvironmentNotFound = errors.New("environment not found")
	ErrEnvironmentDisabled = errors.New("environment disabled")
	ErrSecretUnavailable   = errors.New("secret unavailable")
)

type Environment struct {
	ID        string            `json:"id"`
	SystemID  string            `json:"systemId"`
	Key       string            `json:"key"`
	Name      string            `json:"name"`
	Variables map[string]string `json:"variables"`
	Status    string            `json:"status"`
	CreatedBy string            `json:"createdBy"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}
type SecretReference struct {
	ID            string    `json:"id"`
	SystemID      string    `json:"systemId"`
	EnvironmentID string    `json:"environmentId"`
	VariableKey   string    `json:"variableKey"`
	SecretRef     string    `json:"secretRef,omitempty"`
	CreatedBy     string    `json:"createdBy"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
type SecretProvider interface {
	Resolve(context.Context, string) (string, error)
}
type ResolvedEnvironment struct {
	EnvironmentID string            `json:"environmentId"`
	Variables     map[string]string `json:"-"`
	AllowedHosts  []string          `json:"allowedHosts"`
}

func (r ResolvedEnvironment) String() string {
	return fmt.Sprintf("ResolvedEnvironment{EnvironmentID:%q Variables:[REDACTED] AllowedHosts:%v}", r.EnvironmentID, r.AllowedHosts)
}

func (r ResolvedEnvironment) GoString() string { return r.String() }
