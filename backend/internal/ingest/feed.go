package ingest

import (
	"bufio"
	"context"
	"io"
	"strings"

	"network_monitor/internal/model"
)

// TryEnqueue кладёт строку в общую очередь (тот же путь, что TCP ingest).
// false — очередь полна по depth или bytes, строка дропнута (non-blocking).
//
// Delivery contract: at-most-once / best-effort. Backend не даёт per-line ACK
// и не пишет durable WAL. При open insert circuit workers не dequeue'ят —
// очередь заполняется и дальнейшие TryEnqueue начинают дропать с DroppedTotal.
func (s *Service) TryEnqueue(line, transport string) bool {
	if s == nil || s.lineCh == nil {
		return false
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return true // пустые не считаем drop
	}
	n := int64(len(line))
	maxBytes := int64(s.cfg.QueueMaxBytes)

	if maxBytes > 0 && n > maxBytes {
		s.stats.addDropped(1)
		return false
	}

	// Резервируем байты до send; при полном канале — откат.
	if maxBytes > 0 {
		for {
			cur := s.queueBytes.Load()
			if cur+n > maxBytes {
				s.stats.addDropped(1)
				return false
			}
			if s.queueBytes.CompareAndSwap(cur, cur+n) {
				break
			}
		}
	}

	item := ingestedLine{line: line, transport: transport}
	select {
	case s.lineCh <- item:
		return true
	default:
		if maxBytes > 0 {
			s.queueBytes.Add(-n)
		}
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
