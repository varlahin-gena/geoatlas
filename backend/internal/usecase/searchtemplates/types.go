package searchtemplates

import (
	"errors"
	"strings"
	"unicode/utf8"

	"network_monitor/internal/apperr"
)

const (
	MaxNameLen      = 80
	MaxQueryLen     = 500
	MaxPerUser      = 50
	MaxTemplatesAll = 5000
)

var (
	ErrInvalidName   = apperr.InvalidInput("invalid name")
	ErrInvalidQuery  = apperr.InvalidInput("invalid query")
	ErrNotFound      = apperr.NotFound("not found")
	ErrLimitExceeded = apperr.Conflict("template limit exceeded")
	ErrUnavailable   = errors.New("search templates not configured")
)

// Template — именованный поисковый запрос пользователя.
type Template struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Query     string `json:"query"`
	UpdatedAt string `json:"updated_at"`
}

// TemplateWithAuthor — шаблон с автором (admin overview).
type TemplateWithAuthor struct {
	Template
	Username string `json:"username"`
}

// ValidateNameQuery проверяет лимиты имени и запроса.
func ValidateNameQuery(name, query string) error {
	name = strings.TrimSpace(name)
	query = strings.TrimSpace(query)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		return ErrInvalidName
	}
	if query == "" || utf8.RuneCountInString(query) > MaxQueryLen {
		return ErrInvalidQuery
	}
	return nil
}
