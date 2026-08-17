package reputationcodec

import (
	"io"
	"time"

	"network_monitor/internal/model"
	"network_monitor/internal/reputation"
	usecasereputation "network_monitor/internal/usecase/reputation"
)

// Codec wraps reputation parsers for usecase/reputation.Codec.
type Codec struct{}

func New() *Codec { return &Codec{} }

var _ usecasereputation.Codec = (*Codec)(nil)

func (Codec) ReadCSV(r io.Reader) ([]model.ReputationRange, error) {
	return reputation.ReadCSV(r)
}

func (Codec) ParseNetset(r io.Reader, listName, category, source string) ([]model.ReputationRange, error) {
	return reputation.ParseNetset(r, listName, category, source, time.Now().UTC())
}
