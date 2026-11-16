package anomaly

import (
	"context"
	"strings"
	"unicode/utf8"

	"geoatlas/internal/mapsearch"
)

type excludeCtxKey struct{}

// ExclusionCodes — проверки, для которых можно задать запрос-исключение.
func ExclusionCodes() []string {
	return []string{
		CodePortScan,
		CodeBlockedSurge,
		CodeNewCountryDst,
		CodeRepNewDst,
		CodeByteSurge,
		CodeBeaconing,
		CodeLateralFanout,
	}
}

func knownExclusionCode(code string) bool {
	for _, c := range ExclusionCodes() {
		if c == code {
			return true
		}
	}
	return false
}

// WithExclusions кладёт запросы исключений в контекст скана.
func WithExclusions(ctx context.Context, q map[string]string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(q) == 0 {
		return ctx
	}
	cp := make(map[string]string, len(q))
	for k, v := range q {
		cp[k] = v
	}
	return context.WithValue(ctx, excludeCtxKey{}, cp)
}

// ExclusionQuery — запрос карты, который эта проверка не учитывает.
func ExclusionQuery(ctx context.Context, code string) string {
	if ctx == nil {
		return ""
	}
	q, _ := ctx.Value(excludeCtxKey{}).(map[string]string)
	return strings.TrimSpace(q[code])
}

func cleanExclusions(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for code, raw := range in {
		q, ok := acceptableExclusion(code, raw)
		if !ok {
			continue
		}
		out[code] = q
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func acceptableExclusion(code, raw string) (string, bool) {
	if !knownExclusionCode(code) {
		return "", false
	}
	q := strings.TrimSpace(raw)
	if q == "" || utf8.RuneCountInString(q) > mapsearch.MaxMapQueryRunes {
		return "", false
	}
	if mapsearch.Compile(q).Empty {
		return "", false
	}
	return q, true
}
