package logging

import "testing"

func TestSetupLevelsAndFormats(t *testing.T) {
	for _, level := range []string{"debug", "WARN", "warning", "error", "info", "  ", "TRACE"} {
		Setup(level, "text")
	}
	Setup("info", "json")
	Setup("info", "TEXT")
}
