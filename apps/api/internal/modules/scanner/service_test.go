package scanner

import (
	"bizdevops/apps/api/internal/modules/codesource"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func localSources(systemID string, status string, localPath ...string) *codesource.MemoryRepository {
	r := codesource.NewMemoryRepository()
	path := "/srv/service"
	if len(localPath) > 0 {
		path = localPath[0]
	}
	_ = r.Create(context.Background(), codesource.CodeSource{ID: testSource, SystemID: systemID, Name: "service", SourceType: codesource.TypeLocal, LocalPath: path, Status: status, CreatedBy: testUser})
	return r
}

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
	root := t.TempDir()
	analyzer := AnalyzerFunc(func(string) ([]Operation, error) {
		return []Operation{{Method: "GET", Path: "/orders/:id", Handler: "getOrder", Source: SourceLocation{File: "routes.go", Line: 8}}}, nil
	})
	service := NewService(repo, localSources(testSystemA, codesource.StatusActive, root), analyzer, root)
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil || scan.Status != StatusQueued {
		t.Fatalf("Create scan=%#v err=%v", scan, err)
	}
	if err := service.Run(ctx, testSystemA, scan.ID); err != nil {
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
	if err := service.Run(ctx, testSystemA, scan2.ID); err != nil {
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
	root := t.TempDir()
	wantErr := errors.New("parser exploded")
	service := NewService(repo, localSources(testSystemA, codesource.StatusActive, root), AnalyzerFunc(func(string) ([]Operation, error) { return nil, wantErr }), root)
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Run(ctx, testSystemA, scan.ID); !errors.Is(err, wantErr) {
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
	service := NewService(repo, localSources(testSystemA, codesource.StatusActive), AnalyzerFunc(func(string) ([]Operation, error) { return nil, nil }))
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Run(ctx, testSystemB, scan.ID); !errors.Is(err, ErrScanNotFound) {
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
	root := t.TempDir()
	wantErr := errors.New("database unavailable")
	repo := failingUpsertRepository{Repository: memory, err: wantErr}
	service := NewService(repo, localSources(testSystemA, codesource.StatusActive, root), AnalyzerFunc(func(string) ([]Operation, error) {
		return []Operation{{Method: "POST", Path: "/orders", Handler: "createOrder"}}, nil
	}), root)
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Run(ctx, testSystemA, scan.ID); !errors.Is(err, wantErr) {
		t.Fatalf("Run error=%v want %v", err, wantErr)
	}
	stored, found, err := memory.GetScan(ctx, testSystemA, scan.ID)
	if err != nil || !found || stored.Status != StatusFailed || stored.ErrorMessage != wantErr.Error() {
		t.Fatalf("failed scan=%#v found=%v err=%v", stored, found, err)
	}
}

func TestServiceUsesOnlySystemScopedActiveCodeSources(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	cross := localSources(testSystemB, codesource.StatusActive)
	service := NewService(repo, cross, AnalyzerFunc(func(string) ([]Operation, error) { return nil, nil }))
	if _, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser}); !errors.Is(err, ErrCodeSourceNotFound) {
		t.Fatalf("cross-system create err=%v", err)
	}
	disabled := localSources(testSystemA, codesource.StatusDisabled)
	service = NewService(repo, disabled, AnalyzerFunc(func(string) ([]Operation, error) { return nil, nil }))
	if _, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser}); !errors.Is(err, ErrCodeSourceDisabled) {
		t.Fatalf("disabled create err=%v", err)
	}
}

func TestServiceRunResolvesRegisteredLocalPathAndRejectsGit(t *testing.T) {
	ctx := context.Background()
	scans := NewMemoryRepository()
	root := t.TempDir()
	sources := localSources(testSystemA, codesource.StatusActive, root)
	gotRoot := ""
	service := NewService(scans, sources, AnalyzerFunc(func(root string) ([]Operation, error) { gotRoot = root; return nil, nil }), root)
	scan, err := service.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Run(ctx, testSystemA, scan.ID); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	if gotRoot != resolvedRoot {
		t.Fatalf("analyzer root=%q want=%q", gotRoot, resolvedRoot)
	}

	gitSources := codesource.NewMemoryRepository()
	_ = gitSources.Create(ctx, codesource.CodeSource{ID: testSource, SystemID: testSystemA, Name: "git", SourceType: codesource.TypeGit, RepositoryURL: "https://git.example.test/repo", Status: codesource.StatusActive})
	gitService := NewService(NewMemoryRepository(), gitSources, AnalyzerFunc(func(string) ([]Operation, error) { t.Fatal("analyzer must not run for git source"); return nil, nil }), root)
	gitScan, err := gitService.Create(ctx, CreateScanInput{SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser})
	if err != nil {
		t.Fatal(err)
	}
	if err := gitService.Run(ctx, testSystemA, gitScan.ID); !errors.Is(err, ErrCodeSourceNotRunnable) {
		t.Fatalf("git run err=%v", err)
	}
}

func TestServiceRunRejectsMissingOrDisabledSourceBeforeStateTransition(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		sources *codesource.MemoryRepository
		want    error
	}{
		{"missing", codesource.NewMemoryRepository(), ErrCodeSourceNotFound},
		{"disabled", localSources(testSystemA, codesource.StatusDisabled), ErrCodeSourceDisabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scans := NewMemoryRepository()
			scan := ScanRun{ID: "scan", SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser, Status: StatusQueued}
			if err := scans.CreateScan(ctx, scan); err != nil {
				t.Fatal(err)
			}
			service := NewService(scans, tc.sources, AnalyzerFunc(func(string) ([]Operation, error) { t.Fatal("analyzer must not run"); return nil, nil }))
			if err := service.Run(ctx, testSystemA, scan.ID); !errors.Is(err, tc.want) {
				t.Fatalf("run err=%v want=%v", err, tc.want)
			}
			stored, found, err := scans.GetScan(ctx, testSystemA, scan.ID)
			if err != nil || !found || stored.Status != StatusQueued {
				t.Fatalf("stored=%#v found=%v err=%v", stored, found, err)
			}
		})
	}
}

func TestServiceRunEnforcesResolvedAllowedRoots(t *testing.T) {
	ctx := context.Background()
	allowed := t.TempDir()
	inside := filepath.Join(allowed, "service")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	missing := filepath.Join(allowed, "missing")
	for _, tc := range []struct {
		name, target string
		roots        []string
	}{
		{"unconfigured", inside, nil},
		{"outside", outside, []string{allowed}},
		{"missing", missing, []string{allowed}},
		{"missing allowed root", inside, []string{filepath.Join(allowed, "not-there")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scans := NewMemoryRepository()
			scan := ScanRun{ID: "scan", SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser, Status: StatusQueued}
			if err := scans.CreateScan(ctx, scan); err != nil {
				t.Fatal(err)
			}
			called := false
			service := NewService(scans, localSources(testSystemA, codesource.StatusActive, tc.target), AnalyzerFunc(func(string) ([]Operation, error) { called = true; return nil, nil }), tc.roots...)
			if err := service.Run(ctx, testSystemA, scan.ID); !errors.Is(err, ErrCodeSourceNotRunnable) || called {
				t.Fatalf("err=%v called=%v", err, called)
			}
		})
	}
}

func TestServiceRunRejectsSymlinkEscape(t *testing.T) {
	ctx := context.Background()
	allowed := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(allowed, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	scans := NewMemoryRepository()
	scan := ScanRun{ID: "scan", SystemID: testSystemA, CodeSourceID: testSource, RequestedBy: testUser, Status: StatusQueued}
	if err := scans.CreateScan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	called := false
	service := NewService(scans, localSources(testSystemA, codesource.StatusActive, link), AnalyzerFunc(func(string) ([]Operation, error) { called = true; return nil, nil }), allowed)
	if err := service.Run(ctx, testSystemA, scan.ID); !errors.Is(err, ErrCodeSourceNotRunnable) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}
