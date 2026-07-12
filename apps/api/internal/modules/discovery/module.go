package discovery

import (
	"net/http"

	"bizdevops/apps/api/internal/modules/modulehttp"
)

func Register(mux *http.ServeMux) {
	modulehttp.RegisterPlaceholder(mux, "/api/v1/discoveries", "discovery")
}
