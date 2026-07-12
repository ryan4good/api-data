package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
)

const scannerMaxRequestBody = 1 << 20

var scannerUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type ScannerHTTPService interface {
	Create(ctx context.Context, input CreateScanInput) (ScanRun, error)
	Run(ctx context.Context, systemID, scanID, root string) error
	ListScans(ctx context.Context, systemID string) ([]ScanRun, error)
	ListOperations(ctx context.Context, systemID string) ([]APIOperation, error)
}

type scannerHTTPHandler struct {
	service ScannerHTTPService
	roles   access.RoleReader
}

type createScanRequest struct {
	CodeSourceID string `json:"codeSourceId"`
	SourceRef    string `json:"sourceRef"`
	SourceCommit string `json:"sourceCommit"`
	Language     string `json:"language"`
	Framework    string `json:"framework"`
}

type runScanRequest struct {
	RepositoryRoot string `json:"repositoryRoot"`
}

// RegisterSystemRoutes registers scanner endpoints without choosing identity
// middleware or repository wiring. The root composition layer owns both.
func RegisterSystemRoutes(mux *http.ServeMux, service ScannerHTTPService, roles access.RoleReader) {
	handler := &scannerHTTPHandler{service: service, roles: roles}
	mux.HandleFunc("/api/v1/systems/{systemId}/scans", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.listScans(w, r)
		case http.MethodPost:
			handler.createScan(w, r)
		default:
			scannerMethodNotAllowed(w)
		}
	})
	mux.HandleFunc("/api/v1/systems/{systemId}/scans/{scanId}/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			scannerMethodNotAllowed(w)
			return
		}
		handler.runScan(w, r)
	})
	mux.HandleFunc("/api/v1/systems/{systemId}/api-operations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			scannerMethodNotAllowed(w)
			return
		}
		handler.listOperations(w, r)
	})
}

func (h *scannerHTTPHandler) listScans(w http.ResponseWriter, r *http.Request) {
	systemID, actor, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	_ = actor
	items, err := h.service.ListScans(r.Context(), systemID)
	if err != nil {
		scannerInternalFailure(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, items)
}

func (h *scannerHTTPHandler) createScan(w http.ResponseWriter, r *http.Request) {
	systemID, actor, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	var request createScanRequest
	if err := decodeScannerJSON(w, r, &request); err != nil || !validUUID(request.CodeSourceID) {
		scannerInvalidRequest(w, "codeSourceId must be a UUID and the JSON body must contain only supported fields")
		return
	}
	scan, err := h.service.Create(r.Context(), CreateScanInput{
		SystemID: systemID, CodeSourceID: request.CodeSourceID, RequestedBy: actor.UserID,
		SourceRef: request.SourceRef, SourceCommit: request.SourceCommit,
		Language: request.Language, Framework: request.Framework,
	})
	if err != nil {
		scannerInternalFailure(w)
		return
	}
	httpresponse.Success(w, http.StatusCreated, scan)
}

func (h *scannerHTTPHandler) runScan(w http.ResponseWriter, r *http.Request) {
	systemID, _, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	scanID := r.PathValue("scanId")
	if !validUUID(scanID) {
		scannerInvalidRequest(w, "scanId must be a UUID")
		return
	}
	var request runScanRequest
	if err := decodeScannerJSON(w, r, &request); err != nil {
		scannerInvalidRequest(w, "repositoryRoot is required and the JSON body must contain only supported fields")
		return
	}
	root := strings.TrimSpace(request.RepositoryRoot)
	if root == "" {
		scannerInvalidRequest(w, "repositoryRoot is required")
		return
	}
	if err := h.service.Run(r.Context(), systemID, scanID, root); err != nil {
		switch {
		case errors.Is(err, ErrScanNotFound):
			httpresponse.Failure(w, http.StatusNotFound, "scan_not_found", "scan was not found", nil)
		case errors.Is(err, ErrInvalidTransition):
			httpresponse.Failure(w, http.StatusConflict, "invalid_scan_status", "scan cannot run from its current status", nil)
		default:
			scannerInternalFailure(w)
		}
		return
	}
	httpresponse.Success(w, http.StatusOK, map[string]string{"scanId": scanID, "status": string(StatusSucceeded)})
}

func (h *scannerHTTPHandler) listOperations(w http.ResponseWriter, r *http.Request) {
	systemID, _, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	items, err := h.service.ListOperations(r.Context(), systemID)
	if err != nil {
		scannerInternalFailure(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, items)
}

func (h *scannerHTTPHandler) authorize(w http.ResponseWriter, r *http.Request, write bool) (string, access.Actor, bool) {
	systemID := r.PathValue("systemId")
	if !validUUID(systemID) {
		scannerInvalidRequest(w, "systemId must be a UUID")
		return "", access.Actor{}, false
	}
	actor, found := access.ActorFromContext(r.Context())
	if !found {
		httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
		return "", access.Actor{}, false
	}
	role, member, err := h.roles.RoleForUser(r.Context(), systemID, actor.UserID)
	if err != nil {
		scannerInternalFailure(w)
		return "", access.Actor{}, false
	}
	if !member {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return "", access.Actor{}, false
	}
	if write && access.Role(role) != access.RoleOwner && access.Role(role) != access.RoleMaintainer {
		httpresponse.Failure(w, http.StatusForbidden, "forbidden", "owner or maintainer role is required", nil)
		return "", access.Actor{}, false
	}
	return systemID, actor, true
}

func decodeScannerJSON(w http.ResponseWriter, r *http.Request, value any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, scannerMaxRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func validUUID(value string) bool {
	return scannerUUIDPattern.MatchString(value)
}

func scannerInvalidRequest(w http.ResponseWriter, message string) {
	httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", message, nil)
}

func scannerInternalFailure(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}

func scannerMethodNotAllowed(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
}

var _ ScannerHTTPService = (*Service)(nil)
