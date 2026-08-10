// Package apperr — общие sentinel-ошибки application-слоя для стабильного HTTP-маппинга.
// Сообщение для клиента остаётся в Error(); kind доступен через errors.Is / Unwrap.
package apperr

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidCSV   = errors.New("invalid csv")
	ErrTooLarge     = errors.New("too large")
)

type marked struct {
	kind error
	msg  string
}

func (e marked) Error() string { return e.msg }
func (e marked) Unwrap() error { return e.kind }

func InvalidInput(msg string) error {
	if msg == "" {
		return ErrInvalidInput
	}
	return marked{kind: ErrInvalidInput, msg: msg}
}

func NotFound(msg string) error {
	if msg == "" {
		return ErrNotFound
	}
	return marked{kind: ErrNotFound, msg: msg}
}

func Conflict(msg string) error {
	if msg == "" {
		return ErrConflict
	}
	return marked{kind: ErrConflict, msg: msg}
}

func TooLarge(msg string) error {
	if msg == "" {
		return ErrTooLarge
	}
	return marked{kind: ErrTooLarge, msg: msg}
}

// InvalidCSV помечает ошибку разбора CSV как клиентскую (HTTP 400).
func InvalidCSV(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrInvalidCSV) {
		return err
	}
	return marked{kind: ErrInvalidCSV, msg: err.Error()}
}

// IsClient reports validation / CSV errors that should map to HTTP 400.
func IsClient(err error) bool {
	return errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrInvalidCSV)
}
