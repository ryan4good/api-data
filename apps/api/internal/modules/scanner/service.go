package scanner

import (
	"bizdevops/apps/api/internal/modules/codesource"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CodeAnalyzer interface {
	Analyze(root string) ([]Operation, error)
}

type AnalyzerFunc func(root string) ([]Operation, error)

func (f AnalyzerFunc) Analyze(root string) ([]Operation, error) { return f(root) }

type CreateScanInput struct {
	SystemID     string
	CodeSourceID string
	RequestedBy  string
	SourceRef    string
	SourceCommit string
	Language     string
	Framework    string
	Config       []byte
}

type Service struct {
	repository   Repository
	codeSources  codesource.Repository
	analyzer     CodeAnalyzer
	now          func() time.Time
	newID        func() (string, error)
	allowedRoots []string
}

func NewService(repository Repository, codeSources codesource.Repository, analyzer CodeAnalyzer, allowedRoots ...string) *Service {
	return &Service{repository: repository, codeSources: codeSources, analyzer: analyzer, now: time.Now, newID: newScannerUUID, allowedRoots: append([]string{}, allowedRoots...)}
}

func (s *Service) Create(ctx context.Context, input CreateScanInput) (ScanRun, error) {
	if input.SystemID == "" || input.CodeSourceID == "" || input.RequestedBy == "" {
		return ScanRun{}, errors.New("systemID, codeSourceID and requestedBy are required")
	}
	source, found, err := s.codeSources.Get(ctx, input.SystemID, input.CodeSourceID)
	if err != nil {
		return ScanRun{}, err
	}
	if !found {
		return ScanRun{}, ErrCodeSourceNotFound
	}
	if source.Status != codesource.StatusActive {
		return ScanRun{}, ErrCodeSourceDisabled
	}
	id, err := s.newID()
	if err != nil {
		return ScanRun{}, fmt.Errorf("generate scan id: %w", err)
	}
	scan := ScanRun{
		ID: id, SystemID: input.SystemID, CodeSourceID: input.CodeSourceID, RequestedBy: input.RequestedBy,
		SourceRef: input.SourceRef, SourceCommit: input.SourceCommit, Language: input.Language, Framework: input.Framework,
		Config: append([]byte(nil), input.Config...),
		Status: StatusQueued, CreatedAt: s.now().UTC(),
	}
	if err := s.repository.CreateScan(ctx, scan); err != nil {
		return ScanRun{}, err
	}
	return scan, nil
}

func (s *Service) Run(ctx context.Context, systemID, scanID string) error {
	scan, found, err := s.repository.GetScan(ctx, systemID, scanID)
	if err != nil {
		return err
	}
	if !found {
		return ErrScanNotFound
	}
	source, sourceFound, err := s.codeSources.Get(ctx, systemID, scan.CodeSourceID)
	if err != nil {
		return err
	}
	if !sourceFound {
		return ErrCodeSourceNotFound
	}
	if source.Status != codesource.StatusActive {
		return ErrCodeSourceDisabled
	}
	if source.SourceType != codesource.TypeLocal || strings.TrimSpace(source.LocalPath) == "" {
		return ErrCodeSourceNotRunnable
	}
	resolvedRoot, err := resolveRunnableRoot(source.LocalPath, s.allowedRoots)
	if err != nil {
		return ErrCodeSourceNotRunnable
	}
	startedAt := s.now().UTC()
	if err := s.repository.TransitionScan(ctx, systemID, scanID, scan.Status, StatusRunning, ScanUpdate{StartedAt: &startedAt}); err != nil {
		return err
	}

	discovered, analyzeErr := s.analyzer.Analyze(resolvedRoot)
	if analyzeErr != nil {
		finishedAt := s.now().UTC()
		transitionErr := s.repository.TransitionScan(ctx, systemID, scanID, StatusRunning, StatusFailed, ScanUpdate{
			StartedAt: &startedAt, FinishedAt: &finishedAt, ErrorMessage: analyzeErr.Error(),
		})
		if transitionErr != nil {
			return errors.Join(analyzeErr, transitionErr)
		}
		return fmt.Errorf("analyze source: %w", analyzeErr)
	}

	operations := make([]APIOperation, 0, len(discovered))
	for _, item := range discovered {
		operation, err := s.toAPIOperation(systemID, scanID, item)
		if err != nil {
			return s.failRun(ctx, systemID, scanID, startedAt, err)
		}
		operations = append(operations, operation)
	}
	if err := s.repository.UpsertOperations(ctx, systemID, scanID, operations); err != nil {
		return s.failRun(ctx, systemID, scanID, startedAt, err)
	}
	summary, _ := json.Marshal(map[string]int{"operationCount": len(operations)})
	finishedAt := s.now().UTC()
	if err := s.repository.TransitionScan(ctx, systemID, scanID, StatusRunning, StatusSucceeded, ScanUpdate{
		StartedAt: &startedAt, FinishedAt: &finishedAt, Summary: summary,
	}); err != nil {
		return err
	}
	return nil
}

func resolveRunnableRoot(target string, allowedRoots []string) (string, error) {
	if len(allowedRoots) == 0 {
		return "", ErrCodeSourceNotRunnable
	}
	targetAbsolute, err := filepath.Abs(target)
	if err != nil {
		return "", ErrCodeSourceNotRunnable
	}
	targetResolved, err := filepath.EvalSymlinks(targetAbsolute)
	if err != nil {
		return "", ErrCodeSourceNotRunnable
	}
	targetInfo, err := os.Stat(targetResolved)
	if err != nil || !targetInfo.IsDir() {
		return "", ErrCodeSourceNotRunnable
	}
	for _, allowed := range allowedRoots {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		allowedAbsolute, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		allowedResolved, err := filepath.EvalSymlinks(allowedAbsolute)
		if err != nil {
			continue
		}
		allowedInfo, err := os.Stat(allowedResolved)
		if err != nil || !allowedInfo.IsDir() {
			continue
		}
		relative, err := filepath.Rel(allowedResolved, targetResolved)
		if err != nil {
			continue
		}
		if relative == "." || (!filepath.IsAbs(relative) && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))) {
			return targetResolved, nil
		}
	}
	return "", ErrCodeSourceNotRunnable
}

func (s *Service) failRun(ctx context.Context, systemID, scanID string, startedAt time.Time, cause error) error {
	finishedAt := s.now().UTC()
	transitionErr := s.repository.TransitionScan(ctx, systemID, scanID, StatusRunning, StatusFailed, ScanUpdate{
		StartedAt: &startedAt, FinishedAt: &finishedAt, ErrorMessage: cause.Error(),
	})
	if transitionErr != nil {
		return errors.Join(cause, transitionErr)
	}
	return cause
}

func (s *Service) toAPIOperation(systemID, scanID string, item Operation) (APIOperation, error) {
	id, err := s.newID()
	if err != nil {
		return APIOperation{}, fmt.Errorf("generate operation id: %w", err)
	}
	method := strings.ToUpper(item.Method)
	key := method + " " + item.Path
	evidence, err := json.Marshal(map[string]any{"handler": item.Handler, "source": item.Source})
	if err != nil {
		return APIOperation{}, fmt.Errorf("encode code evidence: %w", err)
	}
	hashInput := struct {
		Method  string
		Path    string
		Handler string
		File    string
		Line    int
	}{method, item.Path, item.Handler, item.Source.File, item.Source.Line}
	normalized, _ := json.Marshal(hashInput)
	digest := sha256.Sum256(normalized)
	now := s.now().UTC()
	return APIOperation{
		ID: id, SystemID: systemID, ScanRunID: scanID, OperationKey: key, Method: method, Path: item.Path,
		CodeEvidence: evidence, VerificationStatus: "unverified", LifecycleStatus: "active",
		ContentHash: hex.EncodeToString(digest[:]), CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Service) ListScans(ctx context.Context, systemID string) ([]ScanRun, error) {
	if systemID == "" {
		return nil, errors.New("systemID is required")
	}
	return s.repository.ListScans(ctx, systemID)
}

func (s *Service) ListOperations(ctx context.Context, systemID string) ([]APIOperation, error) {
	if systemID == "" {
		return nil, errors.New("systemID is required")
	}
	return s.repository.ListOperations(ctx, systemID)
}

func newScannerUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
