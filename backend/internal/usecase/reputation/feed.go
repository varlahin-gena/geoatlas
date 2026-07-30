package reputation

import "network_monitor/internal/config"

// Feed is the application DTO for a URL-backed reputation list.
type Feed struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Format   string `json:"format"`
}

// CatalogFeeds returns installable presets for the UI catalog
// (default seeds + curated extras; retired excluded).
func CatalogFeeds() []Feed {
	src := config.CatalogReputationFeeds()
	out := make([]Feed, 0, len(src))
	for _, f := range src {
		out = append(out, Feed{
			Name: f.Name, URL: f.URL, Category: f.Category, Format: f.Format,
		})
	}
	return out
}
