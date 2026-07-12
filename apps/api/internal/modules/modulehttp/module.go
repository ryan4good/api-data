package modulehttp

import (
	"net/http"

	"bizdevops/apps/api/internal/httpresponse"
)

// RegisterPlaceholder exposes both a collection path and its nested resource
// paths. Domain teams can replace the handler without changing API composition.
func RegisterPlaceholder(mux *http.ServeMux, path, module string) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpresponse.Failure(w, http.StatusNotImplemented, "not_implemented", "module is not implemented", map[string]string{"module": module})
	})
	mux.Handle(path, handler)
	mux.Handle(path+"/", handler)
}
