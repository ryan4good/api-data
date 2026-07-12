package system

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type handler struct {
	repository Repository
	authorizer *access.Authorizer
}

type systemView struct {
	BusinessSystem
	MyRole access.Role `json:"myRole"`
}

type upsertMemberRequest struct {
	UserID string      `json:"userId"`
	Role   access.Role `json:"role"`
}

func Register(mux *http.ServeMux, repository Repository) {
	h := &handler{repository: repository, authorizer: access.NewAuthorizer(repository)}
	protected := func(fn http.HandlerFunc) http.Handler { return access.DevelopmentIdentity(fn) }
	mux.Handle("GET /api/v1/systems", protected(h.listSystems))
	mux.Handle("GET /api/v1/systems/{systemId}", protected(h.getSystem))
	mux.Handle("GET /api/v1/systems/{systemId}/members", protected(h.listMembers))
	mux.Handle("POST /api/v1/systems/{systemId}/members", protected(h.upsertMember))
}

func (h *handler) listSystems(w http.ResponseWriter, r *http.Request) {
	actor, _ := access.ActorFromContext(r.Context())
	items, err := h.repository.ListAuthorized(r.Context(), actor.UserID)
	if err != nil {
		internalFailure(w)
		return
	}
	views := make([]systemView, 0, len(items))
	for _, item := range items {
		views = append(views, systemView{BusinessSystem: item.System, MyRole: item.Role})
	}
	httpresponse.Success(w, http.StatusOK, views)
}

func (h *handler) getSystem(w http.ResponseWriter, r *http.Request) {
	actor, _ := access.ActorFromContext(r.Context())
	item, found, err := h.repository.GetAuthorized(r.Context(), r.PathValue("systemId"), actor.UserID)
	if err != nil {
		internalFailure(w)
		return
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return
	}
	httpresponse.Success(w, http.StatusOK, systemView{BusinessSystem: item.System, MyRole: item.Role})
}

func (h *handler) listMembers(w http.ResponseWriter, r *http.Request) {
	actor, _ := access.ActorFromContext(r.Context())
	systemID := r.PathValue("systemId")
	if !h.requireOwner(w, r, systemID, actor.UserID) {
		return
	}
	members, err := h.repository.ListMembers(r.Context(), systemID)
	if err != nil {
		internalFailure(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, members)
}

func (h *handler) upsertMember(w http.ResponseWriter, r *http.Request) {
	actor, _ := access.ActorFromContext(r.Context())
	systemID := r.PathValue("systemId")
	if !h.requireOwner(w, r, systemID, actor.UserID) {
		return
	}
	var input upsertMemberRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF || !uuidPattern.MatchString(input.UserID) || !input.Role.Valid() {
		httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", "userId must be a UUID and role must be a supported system role", nil)
		return
	}
	member := Member{SystemID: systemID, UserID: input.UserID, DisplayName: input.UserID, Role: input.Role, Status: MemberActive}
	storedMember, err := h.repository.UpsertMember(r.Context(), member)
	if err != nil {
		internalFailure(w)
		return
	}
	httpresponse.Success(w, http.StatusOK, storedMember)
}

func (h *handler) requireOwner(w http.ResponseWriter, r *http.Request, systemID, userID string) bool {
	err := h.authorizer.RequireOwner(r.Context(), systemID, userID)
	if err == nil {
		return true
	}
	if errors.Is(err, access.ErrForbidden) {
		httpresponse.Failure(w, http.StatusForbidden, "forbidden", "owner role is required", nil)
		return false
	}
	internalFailure(w)
	return false
}

func internalFailure(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}
