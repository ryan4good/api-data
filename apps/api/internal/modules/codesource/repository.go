package codesource

import (
	"context"
	"errors"
)

var (
	ErrNotFound     = errors.New("code source not found")
	ErrNameConflict = errors.New("code source name already exists")
)

type Repository interface {
	Create(context.Context, CodeSource) error
	Update(context.Context, CodeSource) error
	Get(context.Context, string, string) (CodeSource, bool, error)
	List(context.Context, string) ([]CodeSource, error)
}
