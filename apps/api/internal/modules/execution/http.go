package execution

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
)

const maxExecutionBodyBytes = 1 << 20

type executionHTTPHandler struct {
	service *Service
	roles   access.RoleReader
}

type executeHTTPRequest struct {
	ScenarioID        string         `json:"scenarioId"`
	ScenarioVersionID string         `json:"scenarioVersionId"`
	EnvironmentID     string         `json:"environmentId"`
	StopAfterStepID   string         `json:"stopAfterStepId,omitempty"`
	InputVariables    map[string]any `json:"inputVariables,omitempty"`
}

func RegisterSystemRoutes(mux *http.ServeMux, service *Service, roles access.RoleReader) {
	handler := &executionHTTPHandler{service: service, roles: roles}
	mux.HandleFunc("POST /api/v1/systems/{systemId}/scenario-runs", handler.execute)
	mux.HandleFunc("POST /api/v1/systems/{systemId}/scenario-runs/{runId}/steps/{stepId}/retry", handler.retry)
}

func (handler *executionHTTPHandler) execute(w http.ResponseWriter, request *http.Request) {
	actor, ok := handler.authorizeWrite(w, request)
	if !ok {
		return
	}
	var input executeHTTPRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, maxExecutionBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF || input.ScenarioID == "" || input.ScenarioVersionID == "" || input.EnvironmentID == "" {
		executionInvalidRequest(w)
		return
	}
	run, err := handler.service.Execute(request.Context(), ExecuteCommand{
		SystemID: request.PathValue("systemId"), ScenarioID: input.ScenarioID, ScenarioVersionID: input.ScenarioVersionID,
		EnvironmentID: input.EnvironmentID, RequestedBy: actor.UserID, StopAfterStepID: input.StopAfterStepID, InputVariables: input.InputVariables,
	})
	if err != nil {
		writeExecutionError(w, err)
		return
	}
	httpresponse.Success(w, http.StatusCreated, run)
}

func (handler *executionHTTPHandler) retry(w http.ResponseWriter, request *http.Request) {
	actor, ok := handler.authorizeWrite(w, request)
	if !ok {
		return
	}
	result, err := handler.service.RetryStep(request.Context(), RetryCommand{
		SystemID: request.PathValue("systemId"), RunID: request.PathValue("runId"), StepID: request.PathValue("stepId"), RequestedBy: actor.UserID,
	})
	if err != nil {
		writeExecutionError(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, result)
}

func (handler *executionHTTPHandler) authorizeWrite(w http.ResponseWriter, request *http.Request) (access.Actor, bool) {
	actor, ok := access.ActorFromContext(request.Context())
	if !ok {
		httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
		return access.Actor{}, false
	}
	role, found, err := handler.roles.RoleForUser(request.Context(), request.PathValue("systemId"), actor.UserID)
	if err != nil {
		executionInternalFailure(w)
		return access.Actor{}, false
	}
	if !found {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return access.Actor{}, false
	}
	switch access.Role(role) {
	case access.RoleOwner, access.RoleMaintainer, access.RoleRunner:
		return actor, true
	default:
		httpresponse.Failure(w, http.StatusForbidden, "forbidden", "runner role is required", nil)
		return access.Actor{}, false
	}
}

func writeExecutionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrExecutorNotConfigured):
		httpresponse.Failure(w, http.StatusServiceUnavailable, "executor_not_configured", "step executor is not configured", nil)
	case errors.Is(err, ErrInvalidCommand):
		executionInvalidRequest(w)
	case errors.Is(err, ErrRunNotFound):
		httpresponse.Failure(w, http.StatusNotFound, "run_not_found", "scenario run was not found", nil)
	case errors.Is(err, ErrStepNotFound):
		httpresponse.Failure(w, http.StatusNotFound, "step_not_found", "scenario step was not found", nil)
	case errors.Is(err, ErrStepNotExecuted):
		httpresponse.Failure(w, http.StatusConflict, "step_not_executed", "scenario step has not been executed", nil)
	default:
		executionInternalFailure(w)
	}
}

func executionInvalidRequest(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", "valid scenario, version and environment identifiers are required", nil)
}

func executionInternalFailure(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}
