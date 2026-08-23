package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

func actorFromRequest(r *http.Request) string {
	if r == nil {
		return "system"
	}
	if sess, ok := SessionFromContext(r.Context()); ok && strings.TrimSpace(sess.Username) != "" {
		return sess.Username
	}
	return "system"
}

func clientIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return loginthrottle.ClientIP(r)
}

func writeAuditEvent(ctx context.Context, logs *usecaseaudit.Service, e usecaseaudit.AuditEvent) {
	if logs == nil {
		return
	}
	if strings.TrimSpace(e.Actor) == "" {
		e.Actor = "system"
	}
	if err := logs.WriteAudit(ctx, e); err != nil {
		slog.Warn("audit write failed", "action", e.Action, "resource_id", e.ResourceID, "err", err)
	}
}
