package runrecord

import (
	"errors"
	"net/http"

	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
	"bizdevops/apps/api/internal/modules/execution"
)

type runRecordHTTPHandler struct {
	service *QueryService
	roles   access.RoleReader
}

func RegisterSystemRoutes(mux *http.ServeMux, service *QueryService, roles access.RoleReader) {
	handler := &runRecordHTTPHandler{service: service, roles: roles}
	mux.HandleFunc("GET /api/v1/systems/{systemId}/scenario-runs", handler.list)
	mux.HandleFunc("GET /api/v1/systems/{systemId}/scenario-runs/{runId}", handler.get)
}

func (handler *runRecordHTTPHandler) list(w http.ResponseWriter, request *http.Request) {
	if !handler.authorizeRead(w, request) {
		return
	}
	runs, err := handler.service.List(request.Context(), request.PathValue("systemId"))
	if err != nil {
		runRecordError(w, err)
		return
	}
	if runs == nil {
		runs = []execution.Run{}
	}
	httpresponse.Success(w, http.StatusOK, runs)
}

func (handler *runRecordHTTPHandler) get(w http.ResponseWriter, request *http.Request) {
	if !handler.authorizeRead(w, request) {
		return
	}
	run, err := handler.service.Get(request.Context(), request.PathValue("systemId"), request.PathValue("runId"))
	if err != nil {
		runRecordError(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, run)
}

func (handler *runRecordHTTPHandler) authorizeRead(w http.ResponseWriter, request *http.Request) bool {
	actor, ok := access.ActorFromContext(request.Context())
	if !ok {
		httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
		return false
	}
	_, found, err := handler.roles.RoleForUser(request.Context(), request.PathValue("systemId"), actor.UserID)
	if err != nil {
		runRecordInternalFailure(w)
		return false
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return false
	}
	return true
}

func runRecordError(w http.ResponseWriter, err error) {
	if errors.Is(err, execution.ErrRunNotFound) {
		httpresponse.Failure(w, http.StatusNotFound, "run_not_found", "scenario run was not found", nil)
		return
	}
	runRecordInternalFailure(w)
}

func runRecordInternalFailure(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}
