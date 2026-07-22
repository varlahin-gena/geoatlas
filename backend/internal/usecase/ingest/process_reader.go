package ingest

import (
	"bufio"
	"context"
	"io"

	"network_monitor/internal/model"
)

// Service — application API для sync-обработки (HTTP fallback без очереди).
type Service struct {
	deps Deps
}

func New(deps Deps) *Service {
	return &Service{deps: deps}
}

// ProcessReader синхронно парсит и пишет строки (без общей очереди).
func (s *Service) ProcessReader(ctx context.Context, r io.Reader, transport string) (model.IngestStats, error) {
	stats := model.IngestStats{}
	if s == nil {
		return stats, nil
	}
	proc := NewProcessor(s.deps, nil)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		outcome, inserted, err := proc.ProcessLine(ctx, scanner.Text(), transport)
		if err != nil {
			return stats, err
		}
		switch outcome {
		case OutcomeEmpty:
			continue
		case OutcomeParsed:
			stats.Received++
			stats.Parsed++
		case OutcomeSkipped:
			stats.Received++
			stats.Skipped++
		case OutcomeParseError:
			stats.Received++
			stats.ParseErrors++
		}
		stats.Inserted += inserted
	}
	if err := scanner.Err(); err != nil {
		return stats, err
	}
	if n, err := proc.Flush(ctx); err != nil {
		return stats, err
	} else {
		stats.Inserted += n
	}
	return stats, nil
}
