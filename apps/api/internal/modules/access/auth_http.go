package access

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"bizdevops/apps/api/internal/httpresponse"
)

type AuthenticationHTTPOptions struct {
	CookieName   string
	CookieSecure bool
	CookiePath   string
}

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func RegisterAuthentication(mux *http.ServeMux, service *AuthenticationService, repository UserRepository, options AuthenticationHTTPOptions) {
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		var input loginInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
			httpresponse.Failure(w, http.StatusBadRequest, "invalid_request", "request body must contain email and password", nil)
			return
		}
		result, err := service.Login(r.Context(), input.Email, input.Password)
		if err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				httpresponse.Failure(w, http.StatusUnauthorized, "invalid_credentials", "email or password is invalid", nil)
				return
			}
			httpresponse.Failure(w, http.StatusInternalServerError, "authentication_failed", "authentication failed", nil)
			return
		}
		http.SetCookie(w, sessionCookie(options, result.AccessToken, int(result.ExpiresIn)))
		httpresponse.Success(w, http.StatusOK, result)
	})

	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		actor, ok := ActorFromContext(r.Context())
		if !ok {
			authenticationFailure(w)
			return
		}
		user, err := repository.FindByID(r.Context(), actor.UserID)
		if err != nil || user.Status != "active" {
			authenticationFailure(w)
			return
		}
		httpresponse.Success(w, http.StatusOK, user.View())
	})

	mux.HandleFunc("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		cookie := sessionCookie(options, "", -1)
		cookie.Expires = time.Unix(1, 0)
		http.SetCookie(w, cookie)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
}

func sessionCookie(options AuthenticationHTTPOptions, value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: options.CookieName, Value: value, Path: options.CookiePath, MaxAge: maxAge, HttpOnly: true, Secure: options.CookieSecure, SameSite: http.SameSiteStrictMode}
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	httpresponse.Failure(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
}
