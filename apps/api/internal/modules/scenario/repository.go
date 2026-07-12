package scenario

import "context"

type Repository interface {
	Promote(context.Context, string, string, string, string) (PromotionResult, error)
	List(context.Context, string) ([]Scenario, error)
	Get(context.Context, string, string) (Detail, bool, error)
	Update(context.Context, string, string, string, UpdateRequest) (Detail, error)
}
type Service struct{ repository Repository }

func NewService(r Repository) *Service { return &Service{repository: r} }
func (s *Service) Promote(ctx context.Context, systemID, discoveryID, candidateID, userID string) (PromotionResult, error) {
	return s.repository.Promote(ctx, systemID, discoveryID, candidateID, userID)
}
func (s *Service) List(ctx context.Context, systemID string) ([]Scenario, error) {
	return s.repository.List(ctx, systemID)
}
func (s *Service) Get(ctx context.Context, systemID, scenarioID string) (Detail, bool, error) {
	return s.repository.Get(ctx, systemID, scenarioID)
}
func (s *Service) Update(ctx context.Context, systemID, scenarioID, userID string, input UpdateRequest) (Detail, error) {
	if err := ValidateUpdate(input); err != nil {
		return Detail{}, err
	}
	return s.repository.Update(ctx, systemID, scenarioID, userID, input)
}
