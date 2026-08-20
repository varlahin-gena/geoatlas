package reputation

// Feed is the application DTO for a URL-backed reputation list.
type Feed struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Format   string `json:"format"`
}
