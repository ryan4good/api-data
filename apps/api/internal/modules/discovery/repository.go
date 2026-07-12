package discovery

import (
	"context"
	"time"
)

type Repository interface {
	CreateWithCandidates(ctx context.Context, discovery Discovery, candidates []Candidate) error
	ListDiscoveries(ctx context.Context, systemID string) ([]Discovery, error)
	ListCandidates(ctx context.Context, systemID, discoveryID string) ([]Candidate, error)
	ReviewCandidate(ctx context.Context, systemID, candidateID string, status ReviewStatus, note, reviewerID string, reviewedAt time.Time) (Candidate, error)
}
