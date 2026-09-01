package anomaly

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

func fingerprint(code, srcIP, dstIP, extra string, hour time.Time) string {
	hour = hour.UTC().Truncate(time.Hour)
	return fingerprintAt(code, srcIP, dstIP, extra, hour)
}

// fingerprintDay — day-bucketed id (beaconing), чтобы не спамить каждый тик.
func fingerprintDay(code, srcIP, dstIP, extra string, t time.Time) string {
	day := t.UTC().Truncate(24 * time.Hour)
	return fingerprintAt(code, srcIP, dstIP, extra, day)
}

func fingerprintAt(code, srcIP, dstIP, extra string, bucket time.Time) string {
	var b strings.Builder
	b.WriteString(code)
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(srcIP))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(dstIP))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(extra))
	b.WriteByte('|')
	b.WriteString(bucket.UTC().Format(time.RFC3339))
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}

func pairKey(src, dst string) string {
	return src + "|" + dst
}

func displayIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == ZeroIPv4 {
		return ""
	}
	return ip
}
