package runrecord

import (
	"context"

	"bizdevops/apps/api/internal/modules/execution"
)

type Repository interface {
	ListRuns(context.Context, string) ([]execution.Run, error)
	GetRun(context.Context, string, string) (execution.Run, error)
}

type QueryService struct{ repository Repository }

func NewQueryService(repository Repository) *QueryService {
	return &QueryService{repository: repository}
}

func (service *QueryService) List(ctx context.Context, systemID string) ([]execution.Run, error) {
	return service.repository.ListRuns(ctx, systemID)
}

func (service *QueryService) Get(ctx context.Context, systemID, runID string) (execution.Run, error) {
	return service.repository.GetRun(ctx, systemID, runID)
}
