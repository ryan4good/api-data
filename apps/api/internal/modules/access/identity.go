package access

import (
	"context"
	"net/http"
	"strings"

	"bizdevops/apps/api/internal/httpresponse"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// DevelopmentUserHeader is a trust boundary enabled only by explicit
// AUTH_MODE=development composition.
const DevelopmentUserHeader = "X-Dev-User-ID"

type Actor struct {
	UserID string `json:"userId"`
}

type JWTIdentityOptions struct {
	Issuer     string
	Audience   string
	SigningKey string
}

type actorContextKey struct{}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok && actor.UserID != ""
}

func DevelopmentIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ActorFromContext(r.Context()); ok {
			next.ServeHTTP(w, r)
			return
		}
		userID := strings.TrimSpace(r.Header.Get(DevelopmentUserHeader))
		if _, err := uuid.Parse(userID); userID == "" || err != nil {
			httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "development user header is required", nil)
			return
		}
		ctx := context.WithValue(r.Context(), actorContextKey{}, Actor{UserID: userID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func JWTIdentity(options JWTIdentityOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(DevelopmentUserHeader) != "" {
				authenticationFailure(w)
				return
			}
			rawToken, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				authenticationFailure(w)
				return
			}
			token, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
				return []byte(options.SigningKey), nil
			},
				jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
				jwt.WithIssuer(options.Issuer),
				jwt.WithAudience(options.Audience),
				jwt.WithExpirationRequired(),
			)
			if err != nil || !token.Valid {
				authenticationFailure(w)
				return
			}
			subject, err := token.Claims.GetSubject()
			if err != nil {
				authenticationFailure(w)
				return
			}
			if _, err := uuid.Parse(subject); err != nil {
				authenticationFailure(w)
				return
			}
			ctx := context.WithValue(r.Context(), actorContextKey{}, Actor{UserID: subject})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIdentity authenticates credentials when they are present, allowing
// public routes through without credentials. Protected handlers still enforce
// the presence of Actor in their own boundary.
func RequestIdentity(mode string, options JWTIdentityOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch mode {
			case "development":
				if r.Header.Get(DevelopmentUserHeader) == "" {
					next.ServeHTTP(w, r)
					return
				}
				DevelopmentIdentity(next).ServeHTTP(w, r)
			default:
				if r.Header.Get(DevelopmentUserHeader) != "" {
					authenticationFailure(w)
					return
				}
				if r.Header.Get("Authorization") == "" {
					next.ServeHTTP(w, r)
					return
				}
				JWTIdentity(options)(next).ServeHTTP(w, r)
			}
		})
	}
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	returnToken := ""
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		returnToken = parts[1]
	}
	return returnToken, returnToken != ""
}

func authenticationFailure(w http.ResponseWriter) {
	httpresponse.Failure(w, http.StatusUnauthorized, "authentication_required", "valid bearer authentication is required", nil)
}
