package model

import (
	"sort"
	"strings"
)

// allowedActionSet — варианты успешного трафика (UserGate, Fortigate, Cisco, …).
var allowedActionSet = map[string]struct{}{
	"allow": {}, "allowed": {}, "accept": {}, "accepted": {},
	"permit": {}, "permitted": {}, "pass": {},
	// UserGate
	"nat": {}, "forward": {}, "redirect": {}, "decrypt": {},
	"route": {}, "proxy": {}, "mirror": {}, "inspect": {},
	// Fortigate
	"close": {}, "start": {},
	// Cisco
	"built": {}, "teardown": {}, "trust": {}, "monitor": {},
}

// blockedActionSet — варианты заблокированного трафика.
var blockedActionSet = map[string]struct{}{
	"deny": {}, "denied": {}, "drop": {}, "dropped": {},
	"reject": {}, "rejected": {}, "block": {}, "blocked": {},
	"reset": {}, "discard": {}, "discarded": {},
}

func IsAllowedAction(a string) bool {
	_, ok := allowedActionSet[strings.ToLower(strings.TrimSpace(a))]
	return ok
}

func IsBlockedAction(a string) bool {
	_, ok := blockedActionSet[strings.ToLower(strings.TrimSpace(a))]
	return ok
}

func AllowedInClause() string { return inClause(allowedActionSet) }
func BlockedInClause() string { return inClause(blockedActionSet) }

func inClause(set map[string]struct{}) string {
	items := make([]string, 0, len(set))
	for k := range set {
		items = append(items, "'"+k+"'")
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}
