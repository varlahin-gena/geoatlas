package model

import (
	"sort"
	"strings"
)

// Action vocabulary SoT for firewall log actions.
//
// Two policies (do not unify — analytics differ):
//   - Success / daily views (traffic_logs.success, v_traffic_daily): allow-list
//     via AllowedInClause() — unknown actions count as not-success.
//   - Edges MV / map filters (blocked_cnt / allowed_cnt, ActionWhereSQL):
//     block-list negation via BlockedInClause() — anything not blocked and not
//     empty/unknown counts as allowed (including unknown vendor verbs).
//
// Runtime SQL is built from these sets (sqlclause + migrate.Ensure*).
// Ops fallback files under clickhouse/ are regenerated:
//
//	go generate ./internal/model/...
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

//go:generate go run ./genactionsql

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
