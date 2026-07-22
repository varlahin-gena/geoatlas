package httpapi

import (
	"net/http"
)

func (h *ParseHandler) ParseTest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if h.parseTestUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parse test unavailable"})
		return
	}
	result, err := h.parseTestUC.Run(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ParseSamples отдаёт примеры строк по вендорам для кнопок-пресетов.
func (h *ParseHandler) ParseSamples(w http.ResponseWriter, r *http.Request) {
	if h.parseTestUC == nil {
		writeJSON(w, http.StatusOK, map[string][]string{})
		return
	}
	writeJSON(w, http.StatusOK, h.parseTestUC.Samples())
}
