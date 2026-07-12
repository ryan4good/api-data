package access

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	testUserID = "10000000-0000-4000-8000-000000000001"
	testEmail  = "admin@example.test"
)

func TestLoginIssuesVerifiableBearerForActiveUser(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := NewMemoryUserRepository([]User{{
		ID: testUserID, Email: testEmail, DisplayName: "Platform Admin", PasswordHash: string(hash), PlatformRole: "admin", Status: "active",
	}})
	now := time.Date(2026, 7, 12, 4, 0, 0, 0, time.UTC)
	service := NewAuthenticationService(repository, TokenOptions{
		Issuer: "bizdevops", Audience: "bizdevops-api", SigningKey: "unit-test-signing-key", TTL: time.Hour, Now: func() time.Time { return now },
	})

	result, err := service.Login(context.Background(), " ADMIN@example.test ", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if result.TokenType != "Bearer" || result.ExpiresIn != 3600 || result.User.ID != testUserID || result.User.PlatformRole != "admin" {
		t.Fatalf("result=%#v", result)
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(result.AccessToken, claims, func(*jwt.Token) (any, error) { return []byte("unit-test-signing-key"), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("bizdevops"), jwt.WithAudience("bizdevops-api"), jwt.WithExpirationRequired(), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || !token.Valid {
		t.Fatalf("token parse error=%v valid=%v", err, token != nil && token.Valid)
	}
	if claims["sub"] != testUserID || claims["email"] != testEmail {
		t.Fatalf("claims=%#v", claims)
	}
}

func TestLoginRejectsUnknownWrongPasswordAndDisabledWithoutLeakingIdentity(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := NewMemoryUserRepository([]User{
		{ID: testUserID, Email: testEmail, PasswordHash: string(hash), Status: "active"},
		{ID: "10000000-0000-4000-8000-000000000002", Email: "disabled@example.test", PasswordHash: string(hash), Status: "disabled"},
	})
	service := NewAuthenticationService(repository, TokenOptions{Issuer: "issuer", Audience: "audience", SigningKey: "key", TTL: time.Hour})

	for _, input := range []struct{ email, password string }{
		{"missing@example.test", "right-password"},
		{testEmail, "wrong-password"},
		{"disabled@example.test", "right-password"},
	} {
		if _, err := service.Login(context.Background(), input.email, input.password); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login(%q) error=%v, want ErrInvalidCredentials", input.email, err)
		}
	}
}

func TestAuthenticationHTTPContract(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password-123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := NewMemoryUserRepository([]User{{ID: testUserID, Email: testEmail, DisplayName: "Admin", PasswordHash: string(hash), PlatformRole: "admin", Status: "active"}})
	service := NewAuthenticationService(repository, TokenOptions{Issuer: "issuer", Audience: "audience", SigningKey: "signing-key", TTL: time.Hour})
	mux := http.NewServeMux()
	RegisterAuthentication(mux, service, repository, AuthenticationHTTPOptions{CookieName: "test_session", CookieSecure: true, CookiePath: "/"})
	handler := RequestIdentity("jwt", JWTIdentityOptions{Issuer: "issuer", Audience: "audience", SigningKey: "signing-key", CookieName: "test_session"})(mux)

	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"admin@example.test","password":"password-123"}`)))
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	var envelope struct {
		Data LoginResult `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "test_session" || cookies[0].Value != envelope.Data.AccessToken || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("login cookies=%#v", cookies)
	}

	me := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+envelope.Data.AccessToken)
	handler.ServeHTTP(me, request)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"email":"admin@example.test"`) || strings.Contains(me.Body.String(), "password") {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}

	cookieMe := httptest.NewRecorder()
	cookieRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	cookieRequest.AddCookie(cookies[0])
	handler.ServeHTTP(cookieMe, cookieRequest)
	if cookieMe.Code != http.StatusOK {
		t.Fatalf("cookie me status=%d body=%s", cookieMe.Code, cookieMe.Body.String())
	}

	logout := httptest.NewRecorder()
	handler.ServeHTTP(logout, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", logout.Code, logout.Body.String())
	}
	if logout.Body.String() != "{\"data\":null}\n" {
		t.Fatalf("logout body=%q", logout.Body.String())
	}
	logoutCookies := logout.Result().Cookies()
	if len(logoutCookies) != 1 || logoutCookies[0].Name != "test_session" || logoutCookies[0].MaxAge >= 0 || !logoutCookies[0].HttpOnly {
		t.Fatalf("logout cookies=%#v", logoutCookies)
	}

	staleLogin := httptest.NewRecorder()
	staleLoginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"admin@example.test","password":"password-123"}`))
	staleLoginRequest.AddCookie(&http.Cookie{Name: "test_session", Value: "expired-or-rotated-token"})
	handler.ServeHTTP(staleLogin, staleLoginRequest)
	if staleLogin.Code != http.StatusOK {
		t.Fatalf("stale-cookie login status=%d body=%s", staleLogin.Code, staleLogin.Body.String())
	}

	staleLogout := httptest.NewRecorder()
	staleLogoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	staleLogoutRequest.AddCookie(&http.Cookie{Name: "test_session", Value: "expired-or-rotated-token"})
	handler.ServeHTTP(staleLogout, staleLogoutRequest)
	if staleLogout.Code != http.StatusOK {
		t.Fatalf("stale-cookie logout status=%d body=%s", staleLogout.Code, staleLogout.Body.String())
	}

	for _, body := range []string{
		`{"email":"missing@example.test","password":"password-123"}`,
		`{"email":"admin@example.test","password":"wrong"}`,
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body)))
		if recorder.Code != http.StatusUnauthorized || recorder.Body.String() != "{\"error\":{\"code\":\"invalid_credentials\",\"message\":\"email or password is invalid\"}}\n" {
			t.Fatalf("failure status=%d body=%q", recorder.Code, recorder.Body.String())
		}
	}

	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated me status=%d body=%s", unauthenticated.Code, unauthenticated.Body.String())
	}
}
