package access

import (
	"context"
	"errors"
	"strings"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           string
	Email        string
	DisplayName  string
	PasswordHash string
	PlatformRole string
	Status       string
}

type UserView struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	DisplayName  string `json:"displayName"`
	PlatformRole string `json:"platformRole"`
}

func (user User) View() UserView {
	return UserView{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, PlatformRole: user.PlatformRole}
}

type UserRepository interface {
	FindByEmail(context.Context, string) (User, error)
	FindByID(context.Context, string) (User, error)
}

type MemoryUserRepository struct {
	byEmail map[string]User
	byID    map[string]User
}

func NewMemoryUserRepository(users []User) *MemoryUserRepository {
	repository := &MemoryUserRepository{byEmail: make(map[string]User), byID: make(map[string]User)}
	for _, user := range users {
		repository.byEmail[normalizeEmail(user.Email)] = user
		repository.byID[user.ID] = user
	}
	return repository
}

func (repository *MemoryUserRepository) FindByEmail(_ context.Context, email string) (User, error) {
	user, ok := repository.byEmail[normalizeEmail(email)]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (repository *MemoryUserRepository) FindByID(_ context.Context, id string) (User, error) {
	user, ok := repository.byID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
