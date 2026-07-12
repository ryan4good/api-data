package codesource

import (
	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type handler struct {
	repository Repository
	roles      access.RoleReader
}
type sourceRequest struct {
	Name          string   `json:"name"`
	SourceType    string   `json:"sourceType"`
	RepositoryURL string   `json:"repositoryUrl"`
	LocalPath     string   `json:"localPath"`
	DefaultRef    string   `json:"defaultRef"`
	IncludePaths  []string `json:"includePaths"`
	ExcludePaths  []string `json:"excludePaths"`
	CredentialRef string   `json:"credentialRef"`
	Status        string   `json:"status"`
}

func RegisterSystemRoutes(mux *http.ServeMux, repository Repository, roles access.RoleReader) {
	h := &handler{repository: repository, roles: roles}
	mux.HandleFunc("/api/v1/systems/{systemId}/code-sources", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.list(w, r)
		case http.MethodPost:
			h.create(w, r)
		default:
			codeSourceMethod(w)
		}
	})
	mux.HandleFunc("/api/v1/systems/{systemId}/code-sources/{sourceId}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			h.update(w, r)
		} else {
			codeSourceMethod(w)
		}
	})
}
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	systemID, _, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	items, err := h.repository.List(r.Context(), systemID)
	if err != nil {
		codeSourceInternal(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, items)
}
func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	systemID, actor, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	req, ok := decodeSource(w, r)
	if !ok {
		return
	}
	source := req.source(systemID, uuid.NewString(), actor.UserID)
	if err := h.repository.Create(r.Context(), source); !h.writeResult(w, err) {
		return
	}
	created, found, err := h.repository.Get(r.Context(), systemID, source.ID)
	if err != nil || !found {
		codeSourceInternal(w)
		return
	}
	httpresponse.Success(w, http.StatusCreated, created)
}
func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	systemID, _, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	sourceID := r.PathValue("sourceId")
	if !validUUID(sourceID) {
		codeSourceInvalid(w, "sourceId must be a UUID")
		return
	}
	req, ok := decodeSource(w, r)
	if !ok {
		return
	}
	source := req.source(systemID, sourceID, "")
	if err := h.repository.Update(r.Context(), source); !h.writeResult(w, err) {
		return
	}
	updated, found, err := h.repository.Get(r.Context(), systemID, sourceID)
	if err != nil || !found {
		codeSourceInternal(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, updated)
}
func (h *handler) writeResult(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, ErrNameConflict) {
		httpresponse.Failure(w, http.StatusConflict, "code_source_name_conflict", "a code source with this name already exists", nil)
	} else if errors.Is(err, ErrNotFound) {
		httpresponse.Failure(w, http.StatusNotFound, "code_source_not_found", "code source was not found", nil)
	} else {
		codeSourceInternal(w)
	}
	return false
}
func (h *handler) authorize(w http.ResponseWriter, r *http.Request, write bool) (string, access.Actor, bool) {
	systemID := r.PathValue("systemId")
	if !validUUID(systemID) {
		codeSourceInvalid(w, "systemId must be a UUID")
		return "", access.Actor{}, false
	}
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
		return "", access.Actor{}, false
	}
	role, found, err := h.roles.RoleForUser(r.Context(), systemID, actor.UserID)
	if err != nil {
		codeSourceInternal(w)
		return "", access.Actor{}, false
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return "", access.Actor{}, false
	}
	if write && access.Role(role) != access.RoleOwner && access.Role(role) != access.RoleMaintainer {
		httpresponse.Failure(w, http.StatusForbidden, "forbidden", "owner or maintainer role is required", nil)
		return "", access.Actor{}, false
	}
	return systemID, actor, true
}
func decodeSource(w http.ResponseWriter, r *http.Request) (sourceRequest, bool) {
	var req sourceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		codeSourceInvalid(w, "request body must be a single valid JSON object")
		return req, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		codeSourceInvalid(w, "request body must contain a single JSON object")
		return req, false
	}
	req.trim()
	if message := req.validate(); message != "" {
		codeSourceInvalid(w, message)
		return req, false
	}
	return req, true
}
func (r *sourceRequest) trim() {
	r.Name = strings.TrimSpace(r.Name)
	r.RepositoryURL = strings.TrimSpace(r.RepositoryURL)
	r.LocalPath = strings.TrimSpace(r.LocalPath)
	r.DefaultRef = strings.TrimSpace(r.DefaultRef)
	r.CredentialRef = strings.TrimSpace(r.CredentialRef)
	if r.Status == "" {
		r.Status = StatusActive
	}
}
func (r sourceRequest) source(systemID, id, actor string) CodeSource {
	return CodeSource{ID: id, SystemID: systemID, Name: r.Name, SourceType: r.SourceType, RepositoryURL: r.RepositoryURL, LocalPath: r.LocalPath, DefaultRef: r.DefaultRef, IncludePaths: append([]string{}, r.IncludePaths...), ExcludePaths: append([]string{}, r.ExcludePaths...), CredentialRef: r.CredentialRef, Status: r.Status, CreatedBy: actor}
}

var windowsAbsolute = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func (r sourceRequest) validate() string {
	if r.Name == "" || len(r.Name) > 128 {
		return "name is required and must not exceed 128 characters"
	}
	if r.SourceType != TypeGit && r.SourceType != TypeLocal {
		return "sourceType must be git or local"
	}
	if r.Status != StatusActive && r.Status != StatusDisabled {
		return "status must be active or disabled"
	}
	if len(r.RepositoryURL) > 1024 || len(r.LocalPath) > 1024 || len(r.DefaultRef) > 255 || len(r.CredentialRef) > 255 {
		return "one or more fields exceed their maximum length"
	}
	if r.SourceType == TypeGit {
		if r.RepositoryURL == "" || r.LocalPath != "" {
			return "git sources require repositoryUrl and must not set localPath"
		}
		parsed, err := url.Parse(r.RepositoryURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && parsed.Scheme != "ssh") {
			return "repositoryUrl must be an https or ssh URL without embedded credentials"
		}
	}
	if r.SourceType == TypeLocal {
		if r.LocalPath == "" || r.RepositoryURL != "" || r.CredentialRef != "" || (!filepath.IsAbs(r.LocalPath) && !strings.HasPrefix(r.LocalPath, "/") && !windowsAbsolute.MatchString(r.LocalPath)) {
			return "local sources require an absolute localPath and must not set repositoryUrl or credentialRef"
		}
	}
	if r.CredentialRef != "" && !validCredentialRef(r.CredentialRef) {
		return "credentialRef must be an external secret reference"
	}
	for _, paths := range [][]string{r.IncludePaths, r.ExcludePaths} {
		for _, value := range paths {
			clean := filepath.ToSlash(strings.TrimSpace(value))
			if clean == "" || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
				return "includePaths and excludePaths must contain relative paths without traversal"
			}
		}
	}
	return ""
}
func validCredentialRef(value string) bool {
	for _, prefix := range []string{"vault://", "secret://", "aws-secrets://", "gcp-secret://"} {
		if strings.HasPrefix(value, prefix) && len(value) > len(prefix) {
			return true
		}
	}
	return false
}
func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && len(value) == 36 && parsed.String() == strings.ToLower(value)
}
func codeSourceInvalid(w http.ResponseWriter, message string) {
	httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", message, nil)
}
func codeSourceInternal(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}
func codeSourceMethod(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
}
