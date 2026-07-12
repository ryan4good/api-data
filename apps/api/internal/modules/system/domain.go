package system

import (
	"time"

	"bizdevops/apps/api/internal/modules/access"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

type MemberStatus string

const (
	MemberActive   MemberStatus = "active"
	MemberDisabled MemberStatus = "disabled"
)

type BusinessSystem struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type AuthorizedSystem struct {
	System BusinessSystem
	Role   access.Role
}

type Member struct {
	SystemID    string       `json:"-"`
	UserID      string       `json:"userId"`
	DisplayName string       `json:"displayName"`
	Email       string       `json:"email,omitempty"`
	Role        access.Role  `json:"role"`
	Status      MemberStatus `json:"status"`
}
