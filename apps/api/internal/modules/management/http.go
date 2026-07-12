package management

import (
	"log/slog"
	"net/http"
	"regexp"

	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
)

var managementUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func Register(mux *http.ServeMux, repository Repository, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	mux.HandleFunc("GET /api/v1/management/overview", func(w http.ResponseWriter, request *http.Request) {
		actor, ok := access.ActorFromContext(request.Context())
		if !ok {
			httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
			return
		}
		overview, err := repository.Overview(request.Context(), actor.UserID)
		if err != nil {
			logger.Error("load management overview", "error", err)
			httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
			return
		}
		httpresponse.Success(w, http.StatusOK, overview)
	})
	mux.HandleFunc("GET /api/v1/management/systems/{systemId}/overview", func(w http.ResponseWriter, request *http.Request) {
		actor, ok := access.ActorFromContext(request.Context())
		if !ok {
			httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
			return
		}
		systemID := request.PathValue("systemId")
		if !managementUUID.MatchString(systemID) {
			httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", "systemId must be a UUID", nil)
			return
		}
		item, found, err := repository.SystemOverview(request.Context(), actor.UserID, systemID)
		if err != nil {
			logger.Error("load management system overview", "error", err)
			httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
			return
		}
		if !found {
			httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
			return
		}
		httpresponse.Success(w, http.StatusOK, item)
	})
}
