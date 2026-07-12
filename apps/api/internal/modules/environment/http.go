package environment

import (
	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type httpHandler struct {
	repository Repository
	roles      access.RoleReader
}
type environmentRequest struct {
	ID        string            `json:"id"`
	Key       string            `json:"key"`
	Name      string            `json:"name"`
	Variables map[string]string `json:"variables"`
	Status    string            `json:"status"`
}
type secretRequest struct {
	VariableKey string `json:"variableKey"`
	SecretRef   string `json:"secretRef"`
}

func RegisterSystemRoutes(mux *http.ServeMux, repository Repository, roles access.RoleReader) {
	h := &httpHandler{repository: repository, roles: roles}
	mux.HandleFunc("/api/v1/systems/{systemId}/environments", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.list(w, r)
		case http.MethodPost:
			h.upsert(w, r)
		default:
			method(w)
		}
	})
	mux.HandleFunc("/api/v1/systems/{systemId}/environments/{environmentId}/secret-references", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.references(w, r)
		case http.MethodPost:
			h.upsertReference(w, r)
		default:
			method(w)
		}
	})
}
func (h *httpHandler) list(w http.ResponseWriter, r *http.Request) {
	systemID, _, _, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	items, err := h.repository.ListEnvironments(r.Context(), systemID)
	if err != nil {
		internal(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, items)
}
func (h *httpHandler) upsert(w http.ResponseWriter, r *http.Request) {
	systemID, actor, _, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	var req environmentRequest
	if err := strict(w, r, &req); err != nil || strings.TrimSpace(req.Key) == "" || strings.TrimSpace(req.Name) == "" || hasSensitiveVariable(req.Variables) {
		invalid(w, "key/name are required and sensitive values must use secret references")
		return
	}
	if req.ID == "" {
		var err error
		req.ID, err = newUUID()
		if err != nil {
			internal(w)
			return
		}
	}
	if !uuidPattern.MatchString(req.ID) {
		invalid(w, "id must be a UUID")
		return
	}
	if req.Status == "" {
		req.Status = StatusActive
	}
	if req.Status != StatusActive && req.Status != StatusDisabled {
		invalid(w, "invalid environment status")
		return
	}
	e := Environment{ID: req.ID, SystemID: systemID, Key: req.Key, Name: req.Name, Variables: req.Variables, Status: req.Status, CreatedBy: actor.UserID}
	if err := h.repository.UpsertEnvironment(r.Context(), e); err != nil {
		internal(w)
		return
	}
	httpresponse.Success(w, http.StatusCreated, e)
}
func (h *httpHandler) references(w http.ResponseWriter, r *http.Request) {
	systemID, _, role, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	environmentID := r.PathValue("environmentId")
	if !uuidPattern.MatchString(environmentID) {
		invalid(w, "environmentId must be a UUID")
		return
	}
	refs, err := h.repository.ListSecretReferences(r.Context(), systemID, environmentID)
	if err != nil {
		if errors.Is(err, ErrEnvironmentNotFound) {
			httpresponse.Failure(w, http.StatusNotFound, "environment_not_found", "environment was not found", nil)
		} else {
			internal(w)
		}
		return
	}
	if role == access.RoleViewer {
		for i := range refs {
			refs[i].SecretRef = ""
		}
	}
	httpresponse.Success(w, http.StatusOK, refs)
}
func (h *httpHandler) upsertReference(w http.ResponseWriter, r *http.Request) {
	systemID, actor, _, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	environmentID := r.PathValue("environmentId")
	if !uuidPattern.MatchString(environmentID) {
		invalid(w, "environmentId must be a UUID")
		return
	}
	var req secretRequest
	if err := strict(w, r, &req); err != nil || strings.TrimSpace(req.VariableKey) == "" || !validSecretRef(req.SecretRef) {
		invalid(w, "variableKey and an external secret reference are required")
		return
	}
	id, err := newUUID()
	if err != nil {
		internal(w)
		return
	}
	ref := SecretReference{ID: id, SystemID: systemID, EnvironmentID: environmentID, VariableKey: req.VariableKey, SecretRef: req.SecretRef, CreatedBy: actor.UserID}
	if err := h.repository.UpsertSecretReference(r.Context(), ref); err != nil {
		if errors.Is(err, ErrEnvironmentNotFound) {
			httpresponse.Failure(w, http.StatusNotFound, "environment_not_found", "environment was not found", nil)
		} else {
			internal(w)
		}
		return
	}
	httpresponse.Success(w, http.StatusCreated, ref)
}
func (h *httpHandler) authorize(w http.ResponseWriter, r *http.Request, write bool) (string, access.Actor, access.Role, bool) {
	systemID := r.PathValue("systemId")
	if !uuidPattern.MatchString(systemID) {
		invalid(w, "systemId must be a UUID")
		return "", access.Actor{}, "", false
	}
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
		return "", access.Actor{}, "", false
	}
	role, found, err := h.roles.RoleForUser(r.Context(), systemID, actor.UserID)
	if err != nil {
		internal(w)
		return "", access.Actor{}, "", false
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return "", access.Actor{}, "", false
	}
	rr := access.Role(role)
	if write && rr != access.RoleOwner && rr != access.RoleMaintainer {
		httpresponse.Failure(w, http.StatusForbidden, "forbidden", "owner or maintainer role is required", nil)
		return "", access.Actor{}, rr, false
	}
	return systemID, actor, rr, true
}
func strict(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple JSON")
	}
	return nil
}
func hasSensitiveVariable(vars map[string]string) bool {
	for k := range vars {
		lower := strings.ToLower(k)
		for _, word := range []string{"password", "token", "secret", "credential", "api_key", "apikey"} {
			if strings.Contains(lower, word) {
				return true
			}
		}
	}
	return false
}
func validSecretRef(v string) bool {
	for _, prefix := range []string{"vault://", "secret://", "aws-secrets://", "gcp-secret://"} {
		if strings.HasPrefix(v, prefix) && len(v) > len(prefix) {
			return true
		}
	}
	return false
}
func newUUID() (string, error) {
	var v [16]byte
	if _, err := rand.Read(v[:]); err != nil {
		return "", err
	}
	v[6] = (v[6] & 15) | 64
	v[8] = (v[8] & 63) | 128
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", v[0:4], v[4:6], v[6:8], v[8:10], v[10:16]), nil
}
func invalid(w http.ResponseWriter, m string) {
	httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", m, nil)
}
func internal(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}
func method(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
}
