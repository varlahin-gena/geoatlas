package reputation

const fireholRawBase = "https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/"

// DefaultFeeds — отдельные исходные списки (не агрегат level1),
// чтобы в UI было видно тип угрозы: DROP / C2 / scanners и т.п.
// fullbogons намеренно не включён (частные сети и так исключаются в Lookup).
func DefaultFeeds() []Feed {
	return []Feed{
		{
			Name: "spamhaus_drop", URL: fireholRawBase + "spamhaus_drop.netset",
			Category: "drop", Format: "netset",
		},
		{
			Name: "dshield", URL: fireholRawBase + "dshield.netset",
			Category: "attacks", Format: "netset",
		},
		{
			Name: "feodo", URL: fireholRawBase + "feodo.ipset",
			Category: "c2", Format: "netset",
		},
		{
			Name: "et_block", URL: fireholRawBase + "et_block.netset",
			Category: "block", Format: "netset",
		},
		{
			Name: "blocklist_de", URL: fireholRawBase + "blocklist_de.ipset",
			Category: "attacks", Format: "netset",
		},
		{
			Name: "ciarmy", URL: fireholRawBase + "ciarmy.ipset",
			Category: "attacks", Format: "netset",
		},
		{
			Name: "greensnow", URL: fireholRawBase + "greensnow.ipset",
			Category: "attacks", Format: "netset",
		},
		{
			Name: "et_compromised", URL: fireholRawBase + "et_compromised.ipset",
			Category: "c2", Format: "netset",
		},
		{
			Name: "bruteforceblocker", URL: fireholRawBase + "bruteforceblocker.ipset",
			Category: "attacks", Format: "netset",
		},
	}
}

// RetiredFeedNames — upstream удалён (404), deprecated или нестабилен; не сидим и вычищаем из JSON.
var RetiredFeedNames = map[string]struct{}{
	"cruzit_web_attacks": {}, // firehol cruzit_web_attacks.ipset снят (404)
	"sslbl":              {}, // FireHOL sslbl.ipset снят; abuse.ch SSLBL IP list deprecated
	"et_block_official":  {}, // rules.emergingthreats.net часто timeout при fetch
}

// WithoutRetired убирает retired имена; changed=true если что-то отфильтровали.
func WithoutRetired(feeds []Feed) (out []Feed, changed bool) {
	if len(feeds) == 0 {
		return nil, false
	}
	out = make([]Feed, 0, len(feeds))
	for _, f := range feeds {
		if _, retired := RetiredFeedNames[f.Name]; retired {
			changed = true
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, changed
	}
	return out, changed
}

// CatalogFeeds — пресеты для UI «каталог»: все сиды по умолчанию
// плюс дополнительные официальные URL. Retired не включаем.
// Уже активные фиды UI скрывает сам — так удалённый список можно добавить снова.
func CatalogFeeds() []Feed {
	extras := []Feed{
		{
			Name:     "spamhaus_drop_official",
			URL:      "https://www.spamhaus.org/drop/drop_v4.json",
			Category: "drop", Format: "spamhaus_json",
		},
		{
			Name:     "feodo_abusech",
			URL:      "https://feodotracker.abuse.ch/downloads/ipblocklist.txt",
			Category: "c2", Format: "netset",
		},
		{
			Name:     "feodo_badips",
			URL:      fireholRawBase + "feodo_badips.ipset",
			Category: "c2", Format: "netset",
		},
		{
			Name:     "blocklist_de_ssh",
			URL:      "https://lists.blocklist.de/lists/ssh.txt",
			Category: "attacks", Format: "netset",
		},
	}
	seen := map[string]struct{}{}
	out := make([]Feed, 0, len(DefaultFeeds())+len(extras))
	for _, f := range append(append([]Feed{}, DefaultFeeds()...), extras...) {
		if _, retired := RetiredFeedNames[f.Name]; retired {
			continue
		}
		if _, ok := seen[f.Name]; ok {
			continue
		}
		seen[f.Name] = struct{}{}
		out = append(out, f)
	}
	return out
}
