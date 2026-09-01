package anomaly

import "time"

// FingerprintForHunt — hour-bucketed id для hunt_threshold (без спама каждый тик).
func FingerprintForHunt(huntID string, t time.Time) string {
	return fingerprintAt(CodeHuntThreshold, "", "", huntID, t.UTC().Truncate(time.Hour))
}
