package importer

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/access"
)

const maxUploadBodyBytes = 1 << 20

type importHTTPHandler struct {
	service *Service
	roles   access.RoleReader
}

type uploadHTTPRequest struct {
	FileName string          `json:"fileName"`
	Document json.RawMessage `json:"document"`
}

type importHTTPView struct {
	ID           string           `json:"id"`
	SystemID     string           `json:"systemId"`
	Format       string           `json:"format"`
	FileName     string           `json:"fileName"`
	ContentHash  string           `json:"contentHash"`
	Status       string           `json:"status"`
	Conversion   StoredConversion `json:"conversion"`
	ErrorMessage string           `json:"errorMessage,omitempty"`
	ImportedBy   string           `json:"importedBy"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

// RegisterSystemRoutes registers importer endpoints below a business-system
// boundary. Authentication middleware is composed by the shared server; each
// handler still rejects a missing Actor defensively.
func RegisterSystemRoutes(mux *http.ServeMux, service *Service, roleReader access.RoleReader) {
	handler := &importHTTPHandler{service: service, roles: roleReader}
	mux.HandleFunc("POST /api/v1/systems/{systemId}/scenario-imports", handler.upload)
	mux.HandleFunc("GET /api/v1/systems/{systemId}/scenario-imports", handler.list)
	mux.HandleFunc("GET /api/v1/systems/{systemId}/scenario-imports/{importId}", handler.get)
	mux.HandleFunc("POST /api/v1/systems/{systemId}/scenario-imports/{importId}/confirm-scripts", handler.confirmScripts)
	mux.HandleFunc("POST /api/v1/systems/{systemId}/scenario-imports/{importId}/apply", handler.apply)
}

func (handler *importHTTPHandler) upload(w http.ResponseWriter, request *http.Request) {
	actor, ok := handler.authorize(w, request, access.RoleOwner, access.RoleMaintainer)
	if !ok {
		return
	}
	var input uploadHTTPRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, maxUploadBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF || strings.TrimSpace(input.FileName) == "" || len(input.FileName) > 512 || len(input.Document) == 0 || string(input.Document) == "null" {
		invalidRequest(w)
		return
	}
	systemID := request.PathValue("systemId")
	outcome, err := handler.service.Upload(request.Context(), UploadCommand{
		SystemID: systemID, ImportedBy: actor.UserID, FileName: input.FileName, Document: input.Document,
		Options: Options{SystemKey: systemID, SystemName: systemID},
	})
	if err != nil {
		writeImporterError(w, err)
		return
	}
	httpresponse.Success(w, http.StatusCreated, toImportView(outcome.Import))
}

func (handler *importHTTPHandler) list(w http.ResponseWriter, request *http.Request) {
	if _, ok := handler.authorize(w, request); !ok {
		return
	}
	items, err := handler.service.List(request.Context(), request.PathValue("systemId"))
	if err != nil {
		writeImporterError(w, err)
		return
	}
	views := make([]importHTTPView, 0, len(items))
	for _, item := range items {
		views = append(views, toImportView(item))
	}
	httpresponse.Success(w, http.StatusOK, views)
}

func (handler *importHTTPHandler) get(w http.ResponseWriter, request *http.Request) {
	if _, ok := handler.authorize(w, request); !ok {
		return
	}
	item, err := handler.service.Get(request.Context(), request.PathValue("systemId"), request.PathValue("importId"))
	if err != nil {
		writeImporterError(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, toImportView(item))
}

func (handler *importHTTPHandler) confirmScripts(w http.ResponseWriter, request *http.Request) {
	actor, ok := handler.authorize(w, request, access.RoleOwner, access.RoleMaintainer, access.RoleReviewer)
	if !ok {
		return
	}
	item, err := handler.service.ConfirmScripts(request.Context(), request.PathValue("systemId"), request.PathValue("importId"), actor.UserID)
	if err != nil {
		writeImporterError(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, toImportView(item))
}

func (handler *importHTTPHandler) apply(w http.ResponseWriter, request *http.Request) {
	if _, ok := handler.authorize(w, request, access.RoleOwner, access.RoleMaintainer); !ok {
		return
	}
	item, err := handler.service.Apply(request.Context(), request.PathValue("systemId"), request.PathValue("importId"))
	if err != nil {
		writeImporterError(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, toImportView(item))
}

func (handler *importHTTPHandler) authorize(w http.ResponseWriter, request *http.Request, allowed ...access.Role) (access.Actor, bool) {
	actor, foundActor := access.ActorFromContext(request.Context())
	if !foundActor {
		httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "authentication is required", nil)
		return access.Actor{}, false
	}
	role, foundMember, err := handler.roles.RoleForUser(request.Context(), request.PathValue("systemId"), actor.UserID)
	if err != nil {
		internalImporterFailure(w)
		return access.Actor{}, false
	}
	if !foundMember {
		httpresponse.Failure(w, http.StatusNotFound, "system_not_found", "system was not found", nil)
		return access.Actor{}, false
	}
	if len(allowed) == 0 {
		return actor, true
	}
	for _, permitted := range allowed {
		if access.Role(role) == permitted {
			return actor, true
		}
	}
	httpresponse.Failure(w, http.StatusForbidden, "forbidden", "insufficient system role", nil)
	return access.Actor{}, false
}

func toImportView(record ImportRecord) importHTTPView {
	return importHTTPView{
		ID: record.ID, SystemID: record.SystemID, Format: record.Format, FileName: record.FileName,
		ContentHash: record.ContentHash, Status: record.Status, Conversion: record.Conversion,
		ErrorMessage: record.ErrorMessage, ImportedBy: record.ImportedBy, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}

func writeImporterError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidUpload):
		invalidRequest(w)
	case errors.Is(err, ErrNotFound):
		httpresponse.Failure(w, http.StatusNotFound, "scenario_import_not_found", "scenario import was not found", nil)
	case errors.Is(err, ErrImportHasErrors):
		httpresponse.Failure(w, http.StatusConflict, "import_has_errors", "scenario import has blocking conversion errors", nil)
	case errors.Is(err, ErrScriptsUnreviewed):
		httpresponse.Failure(w, http.StatusConflict, "scripts_unreviewed", "imported scripts require human confirmation", nil)
	case errors.Is(err, ErrNotReady), errors.Is(err, ErrDuplicateHash):
		httpresponse.Failure(w, http.StatusConflict, "import_not_ready", "scenario import is not ready", nil)
	default:
		internalImporterFailure(w)
	}
}

func invalidRequest(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", "fileName and a supported JSON document are required", nil)
}

func internalImporterFailure(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error", nil)
}
