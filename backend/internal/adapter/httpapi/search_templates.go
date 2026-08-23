package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"network_monitor/internal/usecase/searchtemplates"
)

type SearchTemplatesHandler struct{ *SearchTemplatesDeps }

type searchTemplateRequest struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

func (h *SearchTemplatesHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.searchTemplates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "search templates not configured"})
		return
	}
	username, ok := h.currentUsername(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	list, err := h.searchTemplates.List(username)
	if err != nil {
		writeDomainError(w, "list search templates", err)
		return
	}
	if list == nil {
		list = []searchtemplates.Template{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": list})
}

func (h *SearchTemplatesHandler) CreateMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.searchTemplates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "search templates not configured"})
		return
	}
	username, ok := h.currentUsername(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var req searchTemplateRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	t, err := h.searchTemplates.Create(username, req.Name, req.Query)
	if err != nil {
		writeDomainError(w, "create search template", err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *SearchTemplatesHandler) UpdateMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.searchTemplates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "search templates not configured"})
		return
	}
	username, ok := h.currentUsername(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeBadRequest(w, "missing id")
		return
	}
	var req searchTemplateRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	t, err := h.searchTemplates.Update(username, id, req.Name, req.Query)
	if err != nil {
		writeDomainError(w, "update search template", err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *SearchTemplatesHandler) DeleteMine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.searchTemplates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "search templates not configured"})
		return
	}
	username, ok := h.currentUsername(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeBadRequest(w, "missing id")
		return
	}
	if err := h.searchTemplates.Delete(username, id); err != nil {
		writeDomainError(w, "delete search template", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *SearchTemplatesHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.searchTemplates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "search templates not configured"})
		return
	}
	list, err := h.searchTemplates.ListAll()
	if err != nil {
		writeDomainError(w, "list all search templates", err)
		return
	}
	if list == nil {
		list = []searchtemplates.TemplateWithAuthor{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": list})
}

func (h *SearchTemplatesHandler) currentUsername(r *http.Request) (string, bool) {
	if h != nil && h.cfg.AuthDisabled {
		return "anonymous", true
	}
	if sess, ok := SessionFromContext(r.Context()); ok && strings.TrimSpace(sess.Username) != "" {
		return sess.Username, true
	}
	if h == nil || h.sessions == nil {
		return "", false
	}
	sess, err := SessionFromRequest(r, h.sessions)
	if err != nil || strings.TrimSpace(sess.Username) == "" {
		return "", false
	}
	return sess.Username, true
}
