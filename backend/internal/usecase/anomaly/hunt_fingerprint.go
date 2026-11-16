package anomaly

import (
	"strings"
	"time"
)

// FingerprintForHunt — id одного прогона. Пока кейс не закрыт, повтор не пишется
// отдельно (OpenBySuppression). После закрытия следующий пробой получает новый id.
func FingerprintForHunt(huntID string, t time.Time) string {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return fingerprintAt(CodeHuntThreshold, "", "", huntID, t.UTC().Truncate(time.Second))
}

// HuntSuppressionKey — один открытый кейс на сохранённый поиск.
func HuntSuppressionKey(huntID string) SuppressionKey {
	return SuppressionKey(CodeHuntThreshold + "|hunt|" + strings.TrimSpace(huntID))
}
