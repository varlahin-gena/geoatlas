package ingest

import (
	"bufio"
	"context"
	"io"
	"strings"

	"network_monitor/internal/model"
)

// TryEnqueue кладёт строку в общую очередь (тот же путь, что TCP ingest).
// false — очередь полна, строка дропнута (non-blocking).
func (s *Service) TryEnqueue(line, transport string) bool {
	if s == nil || s.lineCh == nil {
		return false
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return true // пустые не считаем drop
	}
	item := ingestedLine{line: line, transport: transport}
	select {
	case s.lineCh <- item:
		return true
	default:
		s.stats.addDropped(1)
		return false
	}
}

// FeedReader ставит строки в общую очередь workers и сразу возвращает
// request-scoped счётчики: received / queued / dropped.
//
// Parsed / skipped / parse_errors / inserted намеренно 0: они глобальные
// (общие с syslog TCP) и при concurrent upload'ах врут. Обработка — async
// через ту же очередь, что TCP; прогресс смотреть в /api/ingest/stats.
// transport обычно "http"; пустой → "http".
func (s *Service) FeedReader(ctx context.Context, r io.Reader, transport string) (model.IngestStats, error) {
	out := model.IngestStats{}
	if s == nil || s.lineCh == nil {
		return out, nil
	}
	if transport == "" {
		transport = "http"
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		out.Received++
		if s.TryEnqueue(line, transport) {
			out.Queued++
		} else {
			out.Dropped++
		}
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	return out, nil
}
