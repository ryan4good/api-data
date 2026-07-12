package scenario

import (
	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
)

var scenarioUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type ScenarioHTTPService interface {
	Promote(context.Context, string, string, string, string) (PromotionResult, error)
	List(context.Context, string) ([]Scenario, error)
	Get(context.Context, string, string) (Detail, bool, error)
	Update(context.Context, string, string, string, UpdateRequest) (Detail, error)
}
type scenarioHandler struct {
	service ScenarioHTTPService
	roles   access.RoleReader
}

func RegisterSystemRoutes(mux *http.ServeMux, service ScenarioHTTPService, roles access.RoleReader) {
	h := &scenarioHandler{service: service, roles: roles}
	mux.HandleFunc("/api/v1/systems/{systemId}/discoveries/{discoveryId}/candidates/{candidateId}/promote", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			scenarioMethod(w)
			return
		}
		h.promote(w, r)
	})
	mux.HandleFunc("/api/v1/systems/{systemId}/scenarios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			scenarioMethod(w)
			return
		}
		h.list(w, r)
	})
	mux.HandleFunc("/api/v1/systems/{systemId}/scenarios/{scenarioId}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			h.update(w, r)
			return
		}
		if r.Method != http.MethodGet {
			scenarioMethod(w)
			return
		}
		h.get(w, r)
	})
}
func (h *scenarioHandler) update(w http.ResponseWriter, r *http.Request) {
	systemID, actor, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	scenarioID := r.PathValue("scenarioId")
	if !scenarioUUIDPattern.MatchString(scenarioID) {
		scenarioInvalid(w, "scenarioId must be a UUID")
		return
	}
	var input UpdateRequest
	if err := decodeScenarioJSON(w, r, &input); err != nil {
		scenarioInvalid(w, "body must be a valid scenario revision")
		return
	}
	detail, err := h.service.Update(r.Context(), systemID, scenarioID, actor.UserID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidRevision):
			scenarioInvalid(w, err.Error())
		case errors.Is(err, ErrScenarioNotFound):
			httpresponse.Failure(w, http.StatusNotFound, "scenario_not_found", "scenario was not found", nil)
		case errors.Is(err, ErrRevisionConflict):
			httpresponse.Failure(w, http.StatusConflict, "revision_conflict", "scenario changed while the revision was being saved", nil)
		default:
			scenarioInternal(w)
		}
		return
	}
	httpresponse.Success(w, http.StatusOK, detail)
}
func (h *scenarioHandler) promote(w http.ResponseWriter, r *http.Request) {
	systemID, actor, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	discoveryID, candidateID := r.PathValue("discoveryId"), r.PathValue("candidateId")
	if !scenarioUUIDPattern.MatchString(discoveryID) || !scenarioUUIDPattern.MatchString(candidateID) {
		scenarioInvalid(w, "discoveryId and candidateId must be UUIDs")
		return
	}
	var body struct{}
	if err := decodeScenarioJSON(w, r, &body); err != nil {
		scenarioInvalid(w, "body must be an empty JSON object")
		return
	}
	result, err := h.service.Promote(r.Context(), systemID, discoveryID, candidateID, actor.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrCandidateNotFound):
			httpresponse.Failure(w, http.StatusNotFound, "candidate_not_found", "candidate was not found", nil)
		case errors.Is(err, ErrCandidateNotAccepted):
			httpresponse.Failure(w, http.StatusConflict, "candidate_not_accepted", "candidate must be accepted before promotion", nil)
		default:
			scenarioInternal(w)
		}
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	httpresponse.Success(w, status, result)
}
func (h *scenarioHandler) list(w http.ResponseWriter, r *http.Request) {
	systemID, _, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	items, err := h.service.List(r.Context(), systemID)
	if err != nil {
		scenarioInternal(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, items)
}
func (h *scenarioHandler) get(w http.ResponseWriter, r *http.Request) {
	systemID, _, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	scenarioID := r.PathValue("scenarioId")
	if !scenarioUUIDPattern.MatchString(scenarioID) {
		scenarioInvalid(w, "scenarioId must be a UUID")
		return
	}
	detail, found, err := h.service.Get(r.Context(), systemID, scenarioID)
	if err != nil {
		scenarioInternal(w)
		return
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "scenario_not_found", "scenario was not found", nil)
		return
	}
	httpresponse.Success(w, http.StatusOK, detail)
}
func (h *scenarioHandler) authorize(w http.ResponseWriter, r *http.Request, write bool) (string, access.Actor, bool) {
	systemID := r.PathValue("systemId")
	if !scenarioUUIDPattern.MatchString(systemID) {
		scenarioInvalid(w, "systemId must be a UUID")
		return "", access.Actor{}, false
	}
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
		return "", access.Actor{}, false
	}
	role, found, err := h.roles.RoleForUser(r.Context(), systemID, actor.UserID)
	if err != nil {
		scenarioInternal(w)
		return "", access.Actor{}, false
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return "", access.Actor{}, false
	}
	rr := access.Role(role)
	if write && rr != access.RoleOwner && rr != access.RoleMaintainer {
		httpresponse.Failure(w, http.StatusForbidden, "forbidden", "owner or maintainer role is required", nil)
		return "", access.Actor{}, false
	}
	return systemID, actor, true
}
func decodeScenarioJSON(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}
func scenarioInvalid(w http.ResponseWriter, m string) {
	httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", m, nil)
}
func scenarioInternal(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}
func scenarioMethod(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
}

var _ ScenarioHTTPService = (*Service)(nil)
