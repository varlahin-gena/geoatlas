package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"geoatlas/internal/model"
	usecasegeo "geoatlas/internal/usecase/geo"
)

type enterpriseNetDTO struct {
	Network   string    `json:"network"`
	StartIP   uint32    `json:"start_ip"`
	EndIP     uint32    `json:"end_ip"`
	Label     string    `json:"label,omitempty"`
	Country   string    `json:"country,omitempty"`
	Region    string    `json:"region,omitempty"`
	City      string    `json:"city,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type addEnterpriseNetRequest struct {
	Network string `json:"network"`
	Label   string `json:"label"`
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
}

func toEnterpriseNetDTO(n model.EnterpriseNet) enterpriseNetDTO {
	return enterpriseNetDTO{
		Network:   n.Network,
		StartIP:   n.StartIP,
		EndIP:     n.EndIP,
		Label:     n.Label,
		Country:   n.Country,
		Region:    n.Region,
		City:      n.City,
		CreatedAt: n.CreatedAt,
	}
}

func (h *GeoHandler) ListEnterpriseNets(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	items, err := h.geoUC.ListEnterpriseNets(r.Context())
	if err != nil {
		writeDomainError(w, "enterprise nets list failed", err)
		return
	}
	if items == nil {
		items = []model.EnterpriseNet{}
	}
	out := make([]enterpriseNetDTO, 0, len(items))
	for _, n := range items {
		out = append(out, toEnterpriseNetDTO(n))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "count": len(out), "max": usecasegeo.MaxEnterpriseNets, "items": out,
	})
}

func (h *GeoHandler) AddEnterpriseNet(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	var req addEnterpriseNetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	n, err := h.geoUC.AddEnterpriseNet(r.Context(), usecasegeo.AddEnterpriseNetInput{
		Network: req.Network, Label: req.Label, Country: req.Country, Region: req.Region, City: req.City,
	})
	if err != nil {
		writeDomainError(w, "enterprise net add failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "item": toEnterpriseNetDTO(n)})
}

func (h *GeoHandler) DeleteEnterpriseNet(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	start, err1 := strconv.ParseUint(strings.TrimSpace(r.PathValue("start_ip")), 10, 32)
	end, err2 := strconv.ParseUint(strings.TrimSpace(r.PathValue("end_ip")), 10, 32)
	if err1 != nil || err2 != nil {
		writeBadRequest(w, "invalid start_ip/end_ip")
		return
	}
	if err := h.geoUC.DeleteEnterpriseNet(r.Context(), uint32(start), uint32(end)); err != nil {
		writeDomainError(w, "enterprise net delete failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
