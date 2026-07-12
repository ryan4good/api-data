package discovery

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
	"bizdevops/apps/api/internal/modules/scanner"
)

const discoveryMaxBody = 1 << 20

var discoveryUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type DiscoveryHTTPService interface {
	Discover(context.Context, DiscoverInput) (Discovery, []Candidate, error)
	ListDiscoveries(context.Context, string) ([]Discovery, error)
	ListCandidates(context.Context, string, string) ([]Candidate, error)
	Review(context.Context, string, string, ReviewStatus, string, string) (Candidate, error)
}
type httpHandler struct {
	service DiscoveryHTTPService
	roles   access.RoleReader
}
type createRequest struct {
	CodeSourceID string                 `json:"codeSourceId"`
	Type         Type                   `json:"type"`
	Name         string                 `json:"name"`
	PRD          string                 `json:"prd"`
	Prompt       string                 `json:"prompt"`
	Operations   []scanner.APIOperation `json:"operations"`
}
type reviewRequest struct {
	Note string `json:"note"`
}

func RegisterSystemRoutes(mux *http.ServeMux, service DiscoveryHTTPService, roles access.RoleReader) {
	h := &httpHandler{service: service, roles: roles}
	mux.HandleFunc("/api/v1/systems/{systemId}/discoveries", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.list(w, r)
		case http.MethodPost:
			h.create(w, r)
		default:
			methodFailure(w)
		}
	})
	mux.HandleFunc("/api/v1/systems/{systemId}/discoveries/{discoveryId}/candidates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodFailure(w)
			return
		}
		h.candidates(w, r)
	})
	mux.HandleFunc("/api/v1/systems/{systemId}/discoveries/{discoveryId}/candidates/{candidateId}/{decision}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodFailure(w)
			return
		}
		h.review(w, r)
	})
}

func (h *httpHandler) list(w http.ResponseWriter, r *http.Request) {
	systemID, _, ok := h.authorize(w, r, "read")
	if !ok {
		return
	}
	items, err := h.service.ListDiscoveries(r.Context(), systemID)
	if err != nil {
		internalFailure(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, items)
}
func (h *httpHandler) create(w http.ResponseWriter, r *http.Request) {
	systemID, actor, ok := h.authorize(w, r, "write")
	if !ok {
		return
	}
	var req createRequest
	if err := strictJSON(w, r, &req); err != nil || !req.Type.Valid() || strings.TrimSpace(req.Name) == "" || (req.CodeSourceID != "" && !discoveryUUID.MatchString(req.CodeSourceID)) {
		invalid(w, "valid type and name are required; codeSourceId must be a UUID")
		return
	}
	d, candidates, err := h.service.Discover(r.Context(), DiscoverInput{SystemID: systemID, CodeSourceID: req.CodeSourceID, Type: req.Type, Name: req.Name, PRD: req.PRD, Prompt: req.Prompt, Operations: req.Operations, RequestedBy: actor.UserID})
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			invalid(w, err.Error())
		} else {
			internalFailure(w)
		}
		return
	}
	httpresponse.Success(w, http.StatusCreated, map[string]any{"discovery": d, "candidates": candidates})
}
func (h *httpHandler) candidates(w http.ResponseWriter, r *http.Request) {
	systemID, _, ok := h.authorize(w, r, "read")
	if !ok {
		return
	}
	discoveryID := r.PathValue("discoveryId")
	if !discoveryUUID.MatchString(discoveryID) {
		invalid(w, "discoveryId must be a UUID")
		return
	}
	items, err := h.service.ListCandidates(r.Context(), systemID, discoveryID)
	if err != nil {
		internalFailure(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, items)
}
func (h *httpHandler) review(w http.ResponseWriter, r *http.Request) {
	systemID, actor, ok := h.authorize(w, r, "review")
	if !ok {
		return
	}
	discoveryID, candidateID := r.PathValue("discoveryId"), r.PathValue("candidateId")
	if !discoveryUUID.MatchString(discoveryID) || !discoveryUUID.MatchString(candidateID) {
		invalid(w, "discoveryId and candidateId must be UUIDs")
		return
	}
	decision := ReviewStatus(r.PathValue("decision") + "ed")
	if decision != ReviewAccepted && decision != ReviewRejected {
		invalid(w, "decision must be accept or reject")
		return
	}
	var req reviewRequest
	if err := strictJSON(w, r, &req); err != nil {
		invalid(w, "invalid review body")
		return
	}
	items, err := h.service.ListCandidates(r.Context(), systemID, discoveryID)
	if err != nil {
		internalFailure(w)
		return
	}
	found := false
	for _, item := range items {
		if item.ID == candidateID {
			found = true
			break
		}
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "candidate_not_found", "candidate was not found", nil)
		return
	}
	item, err := h.service.Review(r.Context(), systemID, candidateID, decision, req.Note, actor.UserID)
	if err != nil {
		if errors.Is(err, ErrCandidateNotFound) {
			httpresponse.Failure(w, http.StatusNotFound, "candidate_not_found", "candidate was not found", nil)
		} else if errors.Is(err, ErrAlreadyReviewed) {
			httpresponse.Failure(w, http.StatusConflict, "candidate_already_reviewed", "candidate was already reviewed", nil)
		} else {
			internalFailure(w)
		}
		return
	}
	httpresponse.Success(w, http.StatusOK, item)
}

func (h *httpHandler) authorize(w http.ResponseWriter, r *http.Request, action string) (string, access.Actor, bool) {
	systemID := r.PathValue("systemId")
	if !discoveryUUID.MatchString(systemID) {
		invalid(w, "systemId must be a UUID")
		return "", access.Actor{}, false
	}
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
		return "", access.Actor{}, false
	}
	role, found, err := h.roles.RoleForUser(r.Context(), systemID, actor.UserID)
	if err != nil {
		internalFailure(w)
		return "", access.Actor{}, false
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return "", access.Actor{}, false
	}
	rRole := access.Role(role)
	allowed := action == "read" || (action == "write" && (rRole == access.RoleOwner || rRole == access.RoleMaintainer)) || (action == "review" && (rRole == access.RoleReviewer || rRole == access.RoleOwner || rRole == access.RoleMaintainer))
	if !allowed {
		httpresponse.Failure(w, http.StatusForbidden, "forbidden", "insufficient system role", nil)
		return "", access.Actor{}, false
	}
	return systemID, actor, true
}
func strictJSON(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, discoveryMaxBody))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}
func invalid(w http.ResponseWriter, message string) {
	httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", message, nil)
}
func internalFailure(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}
func methodFailure(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
}

var _ DiscoveryHTTPService = (*Service)(nil)
