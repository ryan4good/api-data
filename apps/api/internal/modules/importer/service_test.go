package importer

import (
	"context"
	"errors"
	"testing"
	"time"
)

const (
	testSystemID = "10000000-0000-0000-0000-000000000001"
	testUserID   = "20000000-0000-0000-0000-000000000001"
)

func TestServiceUploadIsSystemScopedAndIdempotent(t *testing.T) {
	repository := NewMemoryRepository()
	service := newTestService(repository)
	document := postmanWithScript()

	first, err := service.Upload(context.Background(), UploadCommand{SystemID: testSystemID, ImportedBy: testUserID, FileName: "orders.json", Document: document})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	second, err := service.Upload(context.Background(), UploadCommand{SystemID: testSystemID, ImportedBy: testUserID, FileName: "renamed.json", Document: document})
	if err != nil {
		t.Fatalf("second Upload() error = %v", err)
	}
	if first.Import.ID != second.Import.ID || !second.Existing {
		t.Fatalf("upload was not idempotent: first=%#v second=%#v", first, second)
	}

	otherSystem := "10000000-0000-0000-0000-000000000002"
	third, err := service.Upload(context.Background(), UploadCommand{SystemID: otherSystem, ImportedBy: testUserID, FileName: "orders.json", Document: document})
	if err != nil {
		t.Fatalf("other-system Upload() error = %v", err)
	}
	if third.Import.ID == first.Import.ID || third.Existing {
		t.Fatalf("hash idempotency leaked across systems: %#v", third)
	}
}

func TestServiceRequiresReviewBeforeApplyingImportedScripts(t *testing.T) {
	repository := NewMemoryRepository()
	service := newTestService(repository)
	upload, err := service.Upload(context.Background(), UploadCommand{SystemID: testSystemID, ImportedBy: testUserID, FileName: "orders.json", Document: postmanWithScript()})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if upload.Import.Status != StatusReady || !upload.Import.RequiresScriptReview() {
		t.Fatalf("unexpected ready import: %#v", upload.Import)
	}

	_, err = service.Apply(context.Background(), testSystemID, upload.Import.ID)
	if !errors.Is(err, ErrScriptsUnreviewed) {
		t.Fatalf("Apply() error = %v, want ErrScriptsUnreviewed", err)
	}
	reviewed, err := service.ConfirmScripts(context.Background(), testSystemID, upload.Import.ID, testUserID)
	if err != nil {
		t.Fatalf("ConfirmScripts() error = %v", err)
	}
	if !reviewed.Conversion.Review.ScriptsConfirmed || reviewed.Conversion.Review.ReviewedBy != testUserID {
		t.Fatalf("review not recorded: %#v", reviewed.Conversion.Review)
	}
	applied, err := service.Apply(context.Background(), testSystemID, upload.Import.ID)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if applied.Status != StatusApplied {
		t.Fatalf("status = %q, want applied", applied.Status)
	}
}

func TestServiceFailedConversionCannotBeApplied(t *testing.T) {
	repository := NewMemoryRepository()
	service := newTestService(repository)
	document := []byte(`{"schemaVersion":"1.0","system":{"key":"oms","name":"OMS"},"scenario":{"key":"bad","name":"Bad","version":1,"variables":[],"steps":[]}}`)

	upload, err := service.Upload(context.Background(), UploadCommand{SystemID: testSystemID, ImportedBy: testUserID, FileName: "bad.json", Document: document})
	if err != nil {
		t.Fatalf("failed conversion should be recorded, got error %v", err)
	}
	if upload.Import.Status != StatusFailed || len(upload.Import.Conversion.Result.Report.Errors) == 0 {
		t.Fatalf("conversion failure not recorded: %#v", upload.Import)
	}
	_, err = service.Apply(context.Background(), testSystemID, upload.Import.ID)
	if !errors.Is(err, ErrImportHasErrors) {
		t.Fatalf("Apply() error = %v, want ErrImportHasErrors", err)
	}
}

func TestServiceListAndGetNeverCrossSystemBoundary(t *testing.T) {
	repository := NewMemoryRepository()
	service := newTestService(repository)
	upload, err := service.Upload(context.Background(), UploadCommand{SystemID: testSystemID, ImportedBy: testUserID, FileName: "orders.json", Document: postmanWithoutScript()})
	if err != nil {
		t.Fatal(err)
	}
	otherSystem := "10000000-0000-0000-0000-000000000002"
	items, err := service.List(context.Background(), otherSystem)
	if err != nil || len(items) != 0 {
		t.Fatalf("other-system list = %#v, %v", items, err)
	}
	_, err = service.Get(context.Background(), otherSystem, upload.Import.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("other-system Get() error = %v", err)
	}
}

func TestServiceResolvesConcurrentHashUniquenessRaceIdempotently(t *testing.T) {
	winner := ImportRecord{ID: "winner", SystemID: testSystemID, ContentHash: "unused"}
	repository := &raceRepository{MemoryRepository: NewMemoryRepository(), winner: winner}
	service := newTestService(repository)
	document := postmanWithoutScript()

	outcome, err := service.Upload(context.Background(), UploadCommand{SystemID: testSystemID, ImportedBy: testUserID, FileName: "orders.json", Document: document})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if !outcome.Existing || outcome.Import.ID != "winner" {
		t.Fatalf("race winner not returned: %#v", outcome)
	}
}

type raceRepository struct {
	*MemoryRepository
	winner  ImportRecord
	created bool
}

func (repository *raceRepository) Create(_ context.Context, record ImportRecord) (ImportRecord, error) {
	repository.created = true
	repository.winner.ContentHash = record.ContentHash
	return ImportRecord{}, errors.New("duplicate scoped content hash")
}

func (repository *raceRepository) FindByHash(ctx context.Context, systemID, hash string) (ImportRecord, bool, error) {
	if repository.created && systemID == repository.winner.SystemID && hash == repository.winner.ContentHash {
		return repository.winner, true, nil
	}
	return ImportRecord{}, false, nil
}

func newTestService(repository Repository) *Service {
	nextID := 0
	return NewService(repository, ServiceOptions{
		NewID: func() string {
			nextID++
			if nextID == 1 {
				return "30000000-0000-0000-0000-000000000001"
			}
			return "30000000-0000-0000-0000-000000000002"
		},
		Now: func() time.Time { return time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC) },
	})
}

func postmanWithScript() []byte {
	return []byte(`{"info":{"name":"Orders","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[{"name":"List","request":{"method":"GET","url":"/orders"},"event":[{"listen":"test","script":{"exec":["pm.response.to.have.status(200);"]}}]}]}`)
}

func postmanWithoutScript() []byte {
	return []byte(`{"info":{"name":"Orders","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[{"name":"List","request":{"method":"GET","url":"/orders"}}]}`)
}
