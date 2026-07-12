package access

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type stubRoles struct {
	role  string
	found bool
	err   error
}

func (s stubRoles) RoleForUser(context.Context, string, string) (string, bool, error) {
	return s.role, s.found, s.err
}

func TestDevelopmentIdentityRequiresAndCentralizesUserHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := ActorFromContext(r.Context())
		if !ok || actor.UserID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
			t.Fatalf("actor=%#v ok=%v", actor, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := DevelopmentIdentity(next)

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing header status=%d, want 401", missing.Code)
	}

	present := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(DevelopmentUserHeader, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	handler.ServeHTTP(present, req)
	if present.Code != http.StatusNoContent {
		t.Fatalf("present header status=%d, want 204", present.Code)
	}
}

func TestAuthorizationRequiresActiveOwner(t *testing.T) {
	authorizer := NewAuthorizer(stubRoles{role: string(RoleOwner), found: true})
	if err := authorizer.RequireOwner(context.Background(), "system-id", "user-id"); err != nil {
		t.Fatalf("owner rejected: %v", err)
	}

	for _, roles := range []stubRoles{
		{role: string(RoleMaintainer), found: true},
		{found: false},
	} {
		if err := NewAuthorizer(roles).RequireOwner(context.Background(), "system-id", "user-id"); err != ErrForbidden {
			t.Fatalf("err=%v, want ErrForbidden", err)
		}
	}
}

func TestJWTIdentityAcceptsValidBearerAndSetsActor(t *testing.T) {
	const userID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	options := JWTIdentityOptions{
		Issuer:     "https://identity.example.test",
		Audience:   "bizdevops-api",
		SigningKey: "test-signing-key-that-is-not-used-in-production",
	}
	token := signedToken(t, options.SigningKey, jwt.MapClaims{
		"iss": options.Issuer, "aud": options.Audience, "sub": userID,
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	handler := JWTIdentity(options)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := ActorFromContext(r.Context())
		if !ok || actor.UserID != userID {
			t.Fatalf("actor=%#v ok=%v", actor, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestJWTIdentityRejectsInvalidClaimsAndCredentials(t *testing.T) {
	const signingKey = "test-signing-key-that-is-not-used-in-production"
	valid := jwt.MapClaims{
		"iss": "issuer", "aud": "audience", "sub": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	options := JWTIdentityOptions{Issuer: "issuer", Audience: "audience", SigningKey: signingKey}

	tests := []struct {
		name   string
		header string
		claims jwt.MapClaims
		key    string
		devID  string
	}{
		{name: "missing bearer"},
		{name: "wrong signature", claims: valid, key: "different-signing-key"},
		{name: "wrong issuer", claims: cloneClaims(valid, "iss", "other"), key: signingKey},
		{name: "wrong audience", claims: cloneClaims(valid, "aud", "other"), key: signingKey},
		{name: "expired", claims: cloneClaims(valid, "exp", time.Now().Add(-time.Minute).Unix()), key: signingKey},
		{name: "missing expiration", claims: cloneClaims(valid, "exp", nil), key: signingKey},
		{name: "non uuid subject", claims: cloneClaims(valid, "sub", "alice"), key: signingKey},
		{name: "development header forbidden", claims: valid, key: signingKey, devID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := JWTIdentity(options)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("invalid identity reached protected handler")
			}))
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.claims != nil {
				request.Header.Set("Authorization", "Bearer "+signedToken(t, tt.key, tt.claims))
			}
			if tt.devID != "" {
				request.Header.Set(DevelopmentUserHeader, tt.devID)
			}
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d, want 401; body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestMeReturnsAuthenticatedIdentity(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	handler := DevelopmentIdentity(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set(DevelopmentUserHeader, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data Actor `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.UserID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("user id=%q", body.Data.UserID)
	}
}

func signedToken(t *testing.T, key string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func cloneClaims(source jwt.MapClaims, key string, value any) jwt.MapClaims {
	cloned := jwt.MapClaims{}
	for name, claim := range source {
		cloned[name] = claim
	}
	if value == nil {
		delete(cloned, key)
	} else {
		cloned[key] = value
	}
	return cloned
}
