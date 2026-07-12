package access

import (
	"context"
	"errors"
)

var ErrForbidden = errors.New("insufficient system role")

type RoleReader interface {
	RoleForUser(ctx context.Context, systemID, userID string) (role string, found bool, err error)
}

type Authorizer struct {
	roles RoleReader
}

func NewAuthorizer(roles RoleReader) *Authorizer {
	return &Authorizer{roles: roles}
}

func (a *Authorizer) RequireOwner(ctx context.Context, systemID, userID string) error {
	role, found, err := a.roles.RoleForUser(ctx, systemID, userID)
	if err != nil {
		return err
	}
	if !found || Role(role) != RoleOwner {
		return ErrForbidden
	}
	return nil
}
