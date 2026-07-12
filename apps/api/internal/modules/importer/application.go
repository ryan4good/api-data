package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusUploaded = "uploaded"
	StatusReady    = "ready"
	StatusFailed   = "failed"
	StatusApplied  = "applied"

	DBFormatScenarioBundle = "scenario_bundle"
	DBFormatPostman21      = "postman_2_1"
)

var (
	ErrNotFound          = errors.New("scenario import not found")
	ErrInvalidUpload     = errors.New("invalid scenario import upload")
	ErrDuplicateHash     = errors.New("scenario import content already exists in system")
	ErrImportHasErrors   = errors.New("scenario import has conversion errors")
	ErrScriptsUnreviewed = errors.New("imported scripts require human confirmation")
	ErrNotReady          = errors.New("scenario import is not ready")
)

type Review struct {
	ScriptsConfirmed bool       `json:"scriptsConfirmed"`
	ReviewedBy       string     `json:"reviewedBy,omitempty"`
	ReviewedAt       *time.Time `json:"reviewedAt,omitempty"`
}

type StoredConversion struct {
	Result Result `json:"result"`
	Review Review `json:"review"`
}

type ImportRecord struct {
	ID           string
	SystemID     string
	Format       string
	FileName     string
	ContentHash  string
	Status       string
	RawDocument  json.RawMessage
	Conversion   StoredConversion
	ErrorMessage string
	ImportedBy   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (record ImportRecord) RequiresScriptReview() bool {
	return record.Conversion.Result.Report.Counts.Scripts > 0
}

type Repository interface {
	Create(context.Context, ImportRecord) (ImportRecord, error)
	FindByHash(context.Context, string, string) (ImportRecord, bool, error)
	List(context.Context, string) ([]ImportRecord, error)
	Get(context.Context, string, string) (ImportRecord, error)
	ConfirmScripts(context.Context, string, string, string, time.Time) (ImportRecord, error)
	Apply(context.Context, string, string, time.Time) (ImportRecord, error)
}

type UploadCommand struct {
	SystemID   string
	ImportedBy string
	FileName   string
	Document   []byte
	Options    Options
}

type UploadOutcome struct {
	Import   ImportRecord
	Existing bool
}

type ServiceOptions struct {
	NewID func() string
	Now   func() time.Time
}

type Service struct {
	repository Repository
	newID      func() string
	now        func() time.Time
}

func NewService(repository Repository, options ServiceOptions) *Service {
	if options.NewID == nil {
		options.NewID = uuid.NewString
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{repository: repository, newID: options.NewID, now: options.Now}
}

func (service *Service) Upload(ctx context.Context, command UploadCommand) (UploadOutcome, error) {
	if strings.TrimSpace(command.SystemID) == "" || strings.TrimSpace(command.ImportedBy) == "" ||
		strings.TrimSpace(command.FileName) == "" || len(command.Document) == 0 {
		return UploadOutcome{}, ErrInvalidUpload
	}
	hashBytes := sha256.Sum256(command.Document)
	hash := hex.EncodeToString(hashBytes[:])
	existing, found, err := service.repository.FindByHash(ctx, command.SystemID, hash)
	if err != nil {
		return UploadOutcome{}, err
	}
	if found {
		return UploadOutcome{Import: existing, Existing: true}, nil
	}

	result, conversionErr := Import(command.Document, command.Options)
	format := databaseFormat(result.SourceFormat)
	if format == "" {
		return UploadOutcome{}, ErrInvalidUpload
	}
	now := service.now().UTC()
	record := ImportRecord{
		ID: service.newID(), SystemID: command.SystemID, Format: format, FileName: command.FileName,
		ContentHash: hash, Status: StatusReady, RawDocument: append(json.RawMessage(nil), command.Document...),
		Conversion: StoredConversion{Result: result}, ImportedBy: command.ImportedBy, CreatedAt: now, UpdatedAt: now,
	}
	if conversionErr != nil || len(result.Report.Errors) > 0 {
		record.Status = StatusFailed
		record.ErrorMessage = "conversion failed"
	}
	created, err := service.repository.Create(ctx, record)
	if err != nil {
		// A concurrent uploader may have won the system-scoped hash uniqueness race.
		// Re-read by scope and hash without exposing the document or database error.
		if winner, found, findErr := service.repository.FindByHash(ctx, command.SystemID, hash); findErr == nil && found {
			return UploadOutcome{Import: winner, Existing: true}, nil
		}
		return UploadOutcome{}, err
	}
	return UploadOutcome{Import: created}, nil
}

func (service *Service) List(ctx context.Context, systemID string) ([]ImportRecord, error) {
	return service.repository.List(ctx, systemID)
}

func (service *Service) Get(ctx context.Context, systemID, importID string) (ImportRecord, error) {
	return service.repository.Get(ctx, systemID, importID)
}

func (service *Service) ConfirmScripts(ctx context.Context, systemID, importID, reviewerID string) (ImportRecord, error) {
	if strings.TrimSpace(reviewerID) == "" {
		return ImportRecord{}, ErrInvalidUpload
	}
	return service.repository.ConfirmScripts(ctx, systemID, importID, reviewerID, service.now().UTC())
}

func (service *Service) Apply(ctx context.Context, systemID, importID string) (ImportRecord, error) {
	return service.repository.Apply(ctx, systemID, importID, service.now().UTC())
}

func databaseFormat(source string) string {
	switch source {
	case FormatScenarioBundle:
		return DBFormatScenarioBundle
	case FormatPostmanCollection21:
		return DBFormatPostman21
	default:
		return ""
	}
}

func validateApply(record ImportRecord) error {
	if len(record.Conversion.Result.Report.Errors) > 0 || record.Status == StatusFailed {
		return ErrImportHasErrors
	}
	if record.Status == StatusApplied {
		return nil
	}
	if record.Status != StatusReady {
		return ErrNotReady
	}
	if record.RequiresScriptReview() && !record.Conversion.Review.ScriptsConfirmed {
		return ErrScriptsUnreviewed
	}
	return nil
}
