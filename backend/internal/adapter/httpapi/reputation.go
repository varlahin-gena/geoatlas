package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	usecasereputation "network_monitor/internal/usecase/reputation"
)

type ReputationHandler struct{ *Deps }

func (h *ReputationHandler) UploadReputation(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	dryRun := isDryRun(r)
	var reader io.Reader
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing file"})
			return
		}
		defer file.Close()
		reader = file
	} else {
		if r.Body == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "empty body"})
			return
		}
		defer r.Body.Close()
		reader = r.Body
	}

	result, err := h.reputationUC.UploadCSV(ctx, reader, dryRun)
	if err != nil {
		if usecasereputation.IsClientError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeInternalError(w, "reputation csv upload failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "dry_run": result.DryRun, "ranges": result.Count, "lists": result.Lists,
	})
}

func (h *ReputationHandler) ListLists(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	lists, err := h.reputationUC.ListLists(r.Context())
	if err != nil {
		writeInternalError(w, "reputation list failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "lists": lists})
}

func (h *ReputationHandler) DeleteList(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	name := mux.Vars(r)["name"]
	if err := h.reputationUC.DeleteList(r.Context(), name); err != nil {
		if usecasereputation.IsClientError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeInternalError(w, "reputation delete failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": name})
}

func (h *ReputationHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	force := strings.EqualFold(r.URL.Query().Get("force"), "1") ||
		strings.EqualFold(r.URL.Query().Get("force"), "true")
	res, err := h.reputationUC.Refresh(ctx, force)
	if err != nil {
		writeInternalError(w, "reputation refresh failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "updated": res.Updated, "skipped": res.Skipped, "failed": res.Failed, "errors": res.Errors,
	})
}

func (h *ReputationHandler) Lookup(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	ip := r.URL.Query().Get("ip")
	hits, err := h.reputationUC.Lookup(ip)
	if err != nil {
		if usecasereputation.IsClientError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeInternalError(w, "reputation lookup failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ip": ip, "hits": hits})
}

func (h *ReputationHandler) ListFeeds(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	feeds, err := h.reputationUC.ListFeeds()
	if err != nil {
		writeInternalError(w, "reputation feeds list failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "feeds": feeds})
}

func (h *ReputationHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "feeds": h.reputationUC.ListCatalog()})
}

func (h *ReputationHandler) AddFeed(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Category string `json:"category"`
		Format   string `json:"format"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	err := h.reputationUC.AddFeed(r.Context(), usecasereputation.Feed{
		Name: req.Name, URL: req.URL, Category: req.Category, Format: req.Format,
	})
	if err != nil {
		if usecasereputation.IsClientError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeInternalError(w, "reputation add feed failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "added": req.Name})
}

func (h *ReputationHandler) RemoveFeed(w http.ResponseWriter, r *http.Request) {
	if h.reputationUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "reputation service unavailable"})
		return
	}
	name := mux.Vars(r)["name"]
	if err := h.reputationUC.RemoveFeed(r.Context(), name); err != nil {
		if usecasereputation.IsClientError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeInternalError(w, "reputation remove feed failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": name})
}
