package execution

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
)

type asyncHTTPHandler struct {
	queue      *QueueService
	authorizer *executionHTTPHandler
}

type enqueueHTTPRequest struct {
	ScenarioID         string         `json:"scenarioId"`
	ScenarioVersionID  string         `json:"scenarioVersionId"`
	EnvironmentID      string         `json:"environmentId"`
	RetryKey           string         `json:"retryKey"`
	StopAfterStepID    string         `json:"stopAfterStepId,omitempty"`
	ExecutionTimeoutMS int64          `json:"executionTimeoutMs,omitempty"`
	InputVariables     map[string]any `json:"inputVariables,omitempty"`
}

func RegisterAsyncRoutes(mux *http.ServeMux, queue *QueueService, roles access.RoleReader) {
	handler := &asyncHTTPHandler{queue: queue, authorizer: &executionHTTPHandler{roles: roles}}
	mux.HandleFunc("POST /api/v1/systems/{systemId}/scenario-run-jobs", handler.enqueue)
	mux.HandleFunc("POST /api/v1/systems/{systemId}/scenario-runs/{runId}/cancel", handler.cancel)
}

func (handler *asyncHTTPHandler) enqueue(w http.ResponseWriter, request *http.Request) {
	actor, ok := handler.authorizer.authorizeWrite(w, request)
	if !ok {
		return
	}
	var input enqueueHTTPRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, maxExecutionBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF || input.ScenarioID == "" || input.ScenarioVersionID == "" || input.EnvironmentID == "" || input.RetryKey == "" || input.ExecutionTimeoutMS < 0 {
		executionInvalidRequest(w)
		return
	}
	outcome, err := handler.queue.Enqueue(request.Context(), EnqueueCommand{
		SystemID: request.PathValue("systemId"), ScenarioID: input.ScenarioID, ScenarioVersionID: input.ScenarioVersionID,
		EnvironmentID: input.EnvironmentID, RequestedBy: actor.UserID, RetryKey: input.RetryKey,
		StopAfterStepID: input.StopAfterStepID, ExecutionTimeout: time.Duration(input.ExecutionTimeoutMS) * time.Millisecond, InputVariables: input.InputVariables,
	})
	if err != nil {
		writeExecutionError(w, err)
		return
	}
	status := http.StatusAccepted
	if outcome.Existing {
		status = http.StatusOK
	}
	httpresponse.Success(w, status, outcome)
}

func (handler *asyncHTTPHandler) cancel(w http.ResponseWriter, request *http.Request) {
	if _, ok := handler.authorizer.authorizeWrite(w, request); !ok {
		return
	}
	run, err := handler.queue.Cancel(request.Context(), request.PathValue("systemId"), request.PathValue("runId"))
	if err != nil {
		if errors.Is(err, ErrRunNotCancellable) {
			httpresponse.Failure(w, http.StatusConflict, "run_not_cancellable", "scenario run cannot be cancelled", nil)
			return
		}
		writeExecutionError(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, run)
}
