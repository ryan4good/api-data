package discovery

import (
	"context"
	"testing"

	"bizdevops/apps/api/internal/modules/scanner"
)

const (
	testSystemA   = "11111111-1111-4111-8111-111111111111"
	testSystemB   = "22222222-2222-4222-8222-222222222222"
	testUser      = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testDiscovery = "33333333-3333-4333-8333-333333333333"
	testCandidate = "44444444-4444-4444-8444-444444444444"
)

func orderOperations(systemID string) []scanner.APIOperation {
	return []scanner.APIOperation{
		{ID: "op-list", SystemID: systemID, OperationKey: "GET /orders", Method: "GET", Path: "/orders"},
		{ID: "op-create", SystemID: systemID, OperationKey: "POST /orders", Method: "POST", Path: "/orders"},
		{ID: "op-get", SystemID: systemID, OperationKey: "GET /orders/:id", Method: "GET", Path: "/orders/:id"},
	}
}

func TestServiceDiscoversP0ScenarioFromCodeWithoutPRD(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)
	discovery, candidates, err := service.Discover(context.Background(), DiscoverInput{
		SystemID: testSystemA, Type: TypeCode, Name: "订单代码发现", RequestedBy: testUser,
		Operations: orderOperations(testSystemA),
	})
	if err != nil {
		t.Fatal(err)
	}
	if discovery.Status != StatusReady || len(candidates) != 1 {
		t.Fatalf("discovery=%#v candidates=%#v", discovery, candidates)
	}
	candidate := candidates[0]
	if candidate.Priority != PriorityP0 || !candidate.RequiresReview || candidate.ReviewStatus != ReviewPending || candidate.Confidence < 0.8 {
		t.Fatalf("candidate=%#v", candidate)
	}
	if len(candidate.Steps) != 3 || candidate.Steps[0].OperationID != "op-list" || candidate.Steps[1].OperationID != "op-create" || candidate.Steps[2].OperationID != "op-get" {
		t.Fatalf("steps=%#v", candidate.Steps)
	}
}

func TestDiscoverySupportsPromptAndMixedEvidence(t *testing.T) {
	service := NewService(NewMemoryRepository())
	_, promptCandidates, err := service.Discover(context.Background(), DiscoverInput{
		SystemID: testSystemA, Type: TypePrompt, Name: "支付", Prompt: "用户支付订单后查询支付结果", RequestedBy: testUser,
	})
	if err != nil || len(promptCandidates) != 1 || promptCandidates[0].Priority != PriorityP0 {
		t.Fatalf("prompt candidates=%#v err=%v", promptCandidates, err)
	}
	_, mixedCandidates, err := service.Discover(context.Background(), DiscoverInput{
		SystemID: testSystemA, Type: TypeMixed, Name: "订单", PRD: "创建订单", Prompt: "下单", RequestedBy: testUser,
		Operations: orderOperations(testSystemA),
	})
	if err != nil || len(mixedCandidates) != 1 || len(mixedCandidates[0].SourceRefs) < 3 {
		t.Fatalf("mixed candidates=%#v err=%v", mixedCandidates, err)
	}
}

func TestRepositoryAndReviewAreSystemScoped(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo)
	discovery, candidates, err := service.Discover(ctx, DiscoverInput{SystemID: testSystemA, Type: TypeCode, Name: "Orders", RequestedBy: testUser, Operations: orderOperations(testSystemA)})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.ListDiscoveries(ctx, testSystemB); err != nil || len(got) != 0 {
		t.Fatalf("cross system discoveries=%#v err=%v", got, err)
	}
	if got, err := service.ListCandidates(ctx, testSystemB, discovery.ID); err != nil || len(got) != 0 {
		t.Fatalf("cross system candidates=%#v err=%v", got, err)
	}
	if _, err := service.Review(ctx, testSystemB, candidates[0].ID, ReviewAccepted, "ok", testUser); err != ErrCandidateNotFound {
		t.Fatalf("cross system Review err=%v", err)
	}
	reviewed, err := service.Review(ctx, testSystemA, candidates[0].ID, ReviewAccepted, "核心链路", testUser)
	if err != nil || reviewed.ReviewStatus != ReviewAccepted || reviewed.ReviewedBy != testUser || reviewed.ReviewedAt == nil {
		t.Fatalf("reviewed=%#v err=%v", reviewed, err)
	}
	if ready, err := service.AllCandidatesReviewed(ctx, testSystemA, discovery.ID); err != nil || !ready {
		t.Fatalf("all reviewed ready=%v err=%v", ready, err)
	}
}

func TestCandidatesCannotBeReadyBeforeEveryHumanDecision(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)
	discovery, _, err := service.Discover(context.Background(), DiscoverInput{SystemID: testSystemA, Type: TypeCode, Name: "Orders", RequestedBy: testUser, Operations: orderOperations(testSystemA)})
	if err != nil {
		t.Fatal(err)
	}
	if ready, err := service.AllCandidatesReviewed(context.Background(), testSystemA, discovery.ID); err != nil || ready {
		t.Fatalf("ready=%v err=%v", ready, err)
	}
}
