package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	usecasehunts "geoatlas/internal/usecase/hunts"
)

type HuntsHandler struct {
	hunts *usecasehunts.Service
}

func NewHuntsHandler(svc *usecasehunts.Service) *HuntsHandler {
	return &HuntsHandler{hunts: svc}
}

func (h *HuntsHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.hunts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "hunts not configured"})
		return
	}
	user, ok := h.username(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	list, err := h.hunts.List(user)
	if err != nil {
		writeDomainError(w, "list hunts", err)
		return
	}
	if list == nil {
		list = []usecasehunts.Hunt{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hunts": list})
}

func (h *HuntsHandler) CreateMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.hunts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "hunts not configured"})
		return
	}
	user, ok := h.username(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var req usecasehunts.Hunt
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	out, err := h.hunts.Create(user, req)
	if err != nil {
		writeDomainError(w, "create hunt", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hunt": out})
}

func (h *HuntsHandler) UpdateMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.hunts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "hunts not configured"})
		return
	}
	user, ok := h.username(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	var req usecasehunts.Hunt
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	out, err := h.hunts.Update(user, id, req)
	if err != nil {
		writeDomainError(w, "update hunt", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hunt": out})
}

func (h *HuntsHandler) DeleteMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.hunts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "hunts not configured"})
		return
	}
	user, ok := h.username(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if err := h.hunts.Delete(user, strings.TrimSpace(r.PathValue("id"))); err != nil {
		writeDomainError(w, "delete hunt", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *HuntsHandler) RunMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.hunts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "hunts not configured"})
		return
	}
	user, ok := h.username(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	timeout := 2 * time.Minute
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	hunt, run, err := h.hunts.RunNow(ctx, user, id)
	if err != nil {
		writeDomainError(w, "run hunt", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hunt": hunt, "run": run})
}

func (h *HuntsHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.hunts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "hunts not configured"})
		return
	}
	list, err := h.hunts.ListAll()
	if err != nil {
		writeDomainError(w, "list all hunts", err)
		return
	}
	if list == nil {
		list = []usecasehunts.HuntWithAuthor{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hunts": list})
}

func (h *HuntsHandler) username(r *http.Request) (string, bool) {
	if sess, ok := SessionFromContext(r.Context()); ok && strings.TrimSpace(sess.Username) != "" {
		return sess.Username, true
	}
	return "", false
}
