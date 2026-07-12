// Package access owns identity, authentication, membership, roles, and policy enforcement.
package access

import (
	"net/http"

	"bizdevops/apps/api/internal/httpresponse"
	"bizdevops/apps/api/internal/modules/modulehttp"
)

func Register(mux *http.ServeMux) {
	modulehttp.RegisterPlaceholder(mux, "/api/v1/access", "identity/access")
	mux.HandleFunc("/api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		actor, ok := ActorFromContext(r.Context())
		if !ok {
			authenticationFailure(w)
			return
		}
		httpresponse.Success(w, http.StatusOK, actor)
	})
}
