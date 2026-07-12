package management

import "context"

type Repository interface {
	Overview(ctx context.Context, userID string) (Overview, error)
	SystemOverview(ctx context.Context, userID, systemID string) (SystemOverview, bool, error)
}
