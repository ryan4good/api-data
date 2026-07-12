package environment

import (
	"context"
	"fmt"

	"bizdevops/apps/api/internal/modules/connector"
)

type Resolver struct {
	repository Repository
	provider   SecretProvider
}

func NewResolver(r Repository, p SecretProvider) *Resolver {
	return &Resolver{repository: r, provider: p}
}
func (r *Resolver) Resolve(ctx context.Context, systemID, environmentID string, globalHosts, connectorHosts []string) (ResolvedEnvironment, error) {
	env, found, err := r.repository.GetEnvironment(ctx, systemID, environmentID)
	if err != nil {
		return ResolvedEnvironment{}, err
	}
	if !found {
		return ResolvedEnvironment{}, ErrEnvironmentNotFound
	}
	if env.Status != StatusActive {
		return ResolvedEnvironment{}, ErrEnvironmentDisabled
	}
	values := map[string]string{}
	for k, v := range env.Variables {
		values[k] = v
	}
	refs, err := r.repository.ListSecretReferences(ctx, systemID, environmentID)
	if err != nil {
		return ResolvedEnvironment{}, err
	}
	for _, ref := range refs {
		value, providerErr := r.provider.Resolve(ctx, ref.SecretRef)
		if providerErr != nil || value == "" {
			return ResolvedEnvironment{}, fmt.Errorf("%w: variable %s", ErrSecretUnavailable, ref.VariableKey)
		}
		values[ref.VariableKey] = value
	}
	return ResolvedEnvironment{EnvironmentID: environmentID, Variables: values, AllowedHosts: connector.NarrowHostAllowlist(globalHosts, connectorHosts)}, nil
}
