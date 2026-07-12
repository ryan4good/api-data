package scanner

import (
	"context"
	"errors"
	"testing"
)

const (
	testSystemA = "11111111-1111-4111-8111-111111111111"
	testSystemB = "22222222-2222-4222-8222-222222222222"
	testSource  = "33333333-3333-4333-8333-333333333333"
	testUser    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func TestMemoryRepositoryRequiresSystemScope(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	scan := ScanRun{ID: "scan-a", SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser, Status: StatusQueued}
	if err := repo.CreateScan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	if _, found, err := repo.GetScan(ctx, testSystemB, scan.ID); err != nil || found {
		t.Fatalf("cross-system GetScan found=%v err=%v", found, err)
	}
	if scans, err := repo.ListScans(ctx, testSystemB); err != nil || len(scans) != 0 {
		t.Fatalf("cross-system ListScans=%#v err=%v", scans, err)
	}
}

func TestServiceRunsScanAndIdempotentlyUpsertsOperations(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	analyzer := AnalyzerFunc(func(string) ([]Operation, error) {
		return []Operation{{Method: "GET", Path: "/orders/:id", Handler: "getOrder", Source: SourceLocation{File: "routes.go", Line: 8}}}, nil
	})
	service := NewService(repo, analyzer)
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil || scan.Status != StatusQueued {
		t.Fatalf("Create scan=%#v err=%v", scan, err)
	}
	if err := service.Run(ctx, testSystemA, scan.ID, "."); err != nil {
		t.Fatal(err)
	}
	stored, found, err := repo.GetScan(ctx, testSystemA, scan.ID)
	if err != nil || !found || stored.Status != StatusSucceeded || stored.StartedAt == nil || stored.FinishedAt == nil {
		t.Fatalf("stored scan=%#v found=%v err=%v", stored, found, err)
	}
	operations, err := service.ListOperations(ctx, testSystemA)
	if err != nil || len(operations) != 1 || operations[0].OperationKey != "GET /orders/:id" || len(operations[0].ContentHash) != 64 {
		t.Fatalf("operations=%#v err=%v", operations, err)
	}

	// A second scan of the same route updates the system-scoped asset instead of duplicating it.
	scan2, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Run(ctx, testSystemA, scan2.ID, "."); err != nil {
		t.Fatal(err)
	}
	operations, err = service.ListOperations(ctx, testSystemA)
	if err != nil || len(operations) != 1 || operations[0].ScanRunID != scan2.ID {
		t.Fatalf("idempotent operations=%#v err=%v", operations, err)
	}
}

func TestServiceMarksRunningScanFailedWhenAnalysisFails(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	wantErr := errors.New("parser exploded")
	service := NewService(repo, AnalyzerFunc(func(string) ([]Operation, error) { return nil, wantErr }))
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Run(ctx, testSystemA, scan.ID, "."); !errors.Is(err, wantErr) {
		t.Fatalf("Run error=%v want wrapped %v", err, wantErr)
	}
	stored, found, err := repo.GetScan(ctx, testSystemA, scan.ID)
	if err != nil || !found || stored.Status != StatusFailed || stored.ErrorMessage == "" || stored.FinishedAt == nil {
		t.Fatalf("failed scan=%#v found=%v err=%v", stored, found, err)
	}
}

func TestServiceCannotRunScanThroughAnotherSystem(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, AnalyzerFunc(func(string) ([]Operation, error) { return nil, nil }))
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Run(ctx, testSystemB, scan.ID, "."); !errors.Is(err, ErrScanNotFound) {
		t.Fatalf("cross-system Run error=%v, want ErrScanNotFound", err)
	}
}

type failingUpsertRepository struct {
	Repository
	err error
}

func (r failingUpsertRepository) UpsertOperations(context.Context, string, string, []APIOperation) error {
	return r.err
}

func TestServiceMarksScanFailedWhenOperationPersistenceFails(t *testing.T) {
	ctx := context.Background()
	memory := NewMemoryRepository()
	wantErr := errors.New("database unavailable")
	repo := failingUpsertRepository{Repository: memory, err: wantErr}
	service := NewService(repo, AnalyzerFunc(func(string) ([]Operation, error) {
		return []Operation{{Method: "POST", Path: "/orders", Handler: "createOrder"}}, nil
	}))
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Run(ctx, testSystemA, scan.ID, "."); !errors.Is(err, wantErr) {
		t.Fatalf("Run error=%v want %v", err, wantErr)
	}
	stored, found, err := memory.GetScan(ctx, testSystemA, scan.ID)
	if err != nil || !found || stored.Status != StatusFailed || stored.ErrorMessage != wantErr.Error() {
		t.Fatalf("failed scan=%#v found=%v err=%v", stored, found, err)
	}
}
