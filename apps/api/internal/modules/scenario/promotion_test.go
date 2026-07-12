package scenario

import (
	"context"
	"errors"
	"testing"

	"bizdevops/apps/api/internal/modules/discovery"
)

const (
	systemA     = "11111111-1111-4111-8111-111111111111"
	systemB     = "22222222-2222-4222-8222-222222222222"
	discoveryID = "33333333-3333-4333-8333-333333333333"
	candidateID = "44444444-4444-4444-8444-444444444444"
	userID      = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func acceptedCandidate() PromotionCandidate {
	bundle := discovery.CandidateBundle{Priority: "P0", RequiresReview: true, SourceRefs: []string{"api-operation:op-list", "prd"}, Steps: []discovery.CandidateStep{{Key: "pre", Name: "查询", OperationID: "55555555-5555-4555-8555-555555555555", Method: "GET", Path: "/orders"}, {Key: "action", Name: "创建", OperationID: "66666666-6666-4666-8666-666666666666", Method: "POST", Path: "/orders"}}}
	return PromotionCandidate{ID: candidateID, SystemID: systemA, DiscoveryID: discoveryID, Key: "create-order", Name: "创建订单", Description: "P0", Bundle: bundle, ReviewStatus: "accepted"}
}

func TestPromotionRequiresAcceptedCandidateAndIsIdempotent(t *testing.T) {
	repo := NewMemoryRepository([]PromotionCandidate{acceptedCandidate()})
	service := NewService(repo)
	first, err := service.Promote(context.Background(), systemA, discoveryID, candidateID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.Scenario.Status != "draft" || first.Scenario.CurrentVersionID == "" || len(first.Steps) != 2 {
		t.Fatalf("first=%#v", first)
	}
	if first.Steps[0].Position != 1 || first.Steps[1].Position != 2 || first.Steps[1].DependsOn[0] != "pre" {
		t.Fatalf("steps=%#v", first.Steps)
	}
	second, err := service.Promote(context.Background(), systemA, discoveryID, candidateID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.Scenario.ID != first.Scenario.ID || second.Version.ID != first.Version.ID {
		t.Fatalf("second=%#v first=%#v", second, first)
	}
}

func TestPromotionRejectsPendingRejectedAndCrossSystemCandidates(t *testing.T) {
	for _, status := range []string{"pending", "rejected"} {
		candidate := acceptedCandidate()
		candidate.ReviewStatus = status
		service := NewService(NewMemoryRepository([]PromotionCandidate{candidate}))
		if _, err := service.Promote(context.Background(), systemA, discoveryID, candidateID, userID); !errors.Is(err, ErrCandidateNotAccepted) {
			t.Fatalf("status=%s err=%v", status, err)
		}
	}
	service := NewService(NewMemoryRepository([]PromotionCandidate{acceptedCandidate()}))
	if _, err := service.Promote(context.Background(), systemB, discoveryID, candidateID, userID); !errors.Is(err, ErrCandidateNotFound) {
		t.Fatalf("cross-system err=%v", err)
	}
}

func TestScenarioListsAndDetailsAreSystemScoped(t *testing.T) {
	repo := NewMemoryRepository([]PromotionCandidate{acceptedCandidate()})
	service := NewService(repo)
	promoted, err := service.Promote(context.Background(), systemA, discoveryID, candidateID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if items, err := service.List(context.Background(), systemB); err != nil || len(items) != 0 {
		t.Fatalf("other items=%#v err=%v", items, err)
	}
	if _, found, err := service.Get(context.Background(), systemB, promoted.Scenario.ID); err != nil || found {
		t.Fatalf("cross detail found=%v err=%v", found, err)
	}
	detail, found, err := service.Get(context.Background(), systemA, promoted.Scenario.ID)
	if err != nil || !found || len(detail.Steps) != 2 || detail.Version.Bundle.Priority != "P0" {
		t.Fatalf("detail=%#v found=%v err=%v", detail, found, err)
	}
}
