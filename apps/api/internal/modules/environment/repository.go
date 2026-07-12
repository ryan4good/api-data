package environment

import "context"

type Repository interface {
	UpsertEnvironment(context.Context, Environment) error
	GetEnvironment(context.Context, string, string) (Environment, bool, error)
	ListEnvironments(context.Context, string) ([]Environment, error)
	UpsertSecretReference(context.Context, SecretReference) error
	ListSecretReferences(context.Context, string, string) ([]SecretReference, error)
}
