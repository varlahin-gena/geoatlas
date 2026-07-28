package reputation

// Feed is the application DTO for a URL-backed reputation list.
type Feed struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Format   string `json:"format"`
}

// CatalogFeeds returns curated feed presets shown by the API.
func CatalogFeeds() []Feed {
	return []Feed{
		{Name: "spamhaus_drop_official", URL: "https://www.spamhaus.org/drop/drop_v4.json", Category: "drop", Format: "spamhaus_json"},
		{Name: "feodo_abusech", URL: "https://feodotracker.abuse.ch/downloads/ipblocklist.txt", Category: "c2", Format: "netset"},
		{Name: "feodo_badips", URL: "https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/feodo_badips.ipset", Category: "c2", Format: "netset"},
		{Name: "blocklist_de_ssh", URL: "https://lists.blocklist.de/lists/ssh.txt", Category: "attacks", Format: "netset"},
		{Name: "et_block_official", URL: "https://rules.emergingthreats.net/fwrules/emerging-Block-IPs.txt", Category: "block", Format: "netset"},
	}
}
