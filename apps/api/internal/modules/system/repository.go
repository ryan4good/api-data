package system

import "context"

type Repository interface {
	ListAuthorized(ctx context.Context, userID string) ([]AuthorizedSystem, error)
	GetAuthorized(ctx context.Context, systemID, userID string) (AuthorizedSystem, bool, error)
	RoleForUser(ctx context.Context, systemID, userID string) (string, bool, error)
	ListMembers(ctx context.Context, systemID string) ([]Member, error)
	UpsertMember(ctx context.Context, member Member) (Member, error)
}
