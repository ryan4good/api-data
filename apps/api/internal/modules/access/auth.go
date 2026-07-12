package access

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

var dummyPasswordHash = func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("bizdevops-invalid-credential-sentinel"), bcrypt.DefaultCost)
	return hash
}()

type TokenOptions struct {
	Issuer     string
	Audience   string
	SigningKey string
	TTL        time.Duration
	Now        func() time.Time
}

type AuthenticationService struct {
	repository UserRepository
	options    TokenOptions
}

type LoginResult struct {
	AccessToken string   `json:"accessToken"`
	TokenType   string   `json:"tokenType"`
	ExpiresIn   int64    `json:"expiresIn"`
	User        UserView `json:"user"`
}

type tokenClaims struct {
	Email        string `json:"email"`
	DisplayName  string `json:"displayName"`
	PlatformRole string `json:"platformRole"`
	jwt.RegisteredClaims
}

func NewAuthenticationService(repository UserRepository, options TokenOptions) *AuthenticationService {
	if options.Now == nil {
		options.Now = time.Now
	}
	return &AuthenticationService{repository: repository, options: options}
}

func (service *AuthenticationService) Login(ctx context.Context, email, password string) (LoginResult, error) {
	user, err := service.repository.FindByEmail(ctx, normalizeEmail(email))
	hash := dummyPasswordHash
	if err == nil && user.PasswordHash != "" {
		hash = []byte(user.PasswordHash)
	}
	passwordMatches := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return LoginResult{}, err
	}
	if err != nil || !passwordMatches || user.Status != "active" {
		return LoginResult{}, ErrInvalidCredentials
	}

	now := service.options.Now().UTC()
	expiresAt := now.Add(service.options.TTL)
	claims := tokenClaims{
		Email: user.Email, DisplayName: user.DisplayName, PlatformRole: user.PlatformRole,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: service.options.Issuer, Subject: user.ID, Audience: jwt.ClaimStrings{service.options.Audience},
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(service.options.SigningKey))
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{AccessToken: signed, TokenType: "Bearer", ExpiresIn: int64(service.options.TTL / time.Second), User: user.View()}, nil
}
