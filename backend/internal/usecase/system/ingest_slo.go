package system

// IngestSLO — пороги capacity/health для алёртов и classifyIngest.
// Delivery contract остаётся at-most-once; SLO описывает, когда потери — инцидент.
type IngestSLO struct {
	QueueWarnRatio     float64 `json:"queue_warn_ratio"`
	QueueCriticalRatio float64 `json:"queue_critical_ratio"`

	BufferWarnLines     float64 `json:"buffer_warn_lines"`
	BufferCriticalLines float64 `json:"buffer_critical_lines"`

	// DropsWarnPerSec: любое превышение (обычно 0 → warn при любом drops/s).
	DropsWarnPerSec     float64 `json:"drops_warn_per_sec"`
	DropsCriticalPerSec float64 `json:"drops_critical_per_sec"`

	LagWarnSec     float64 `json:"lag_warn_sec"`
	LagCriticalSec float64 `json:"lag_critical_sec"`

	// CapacityWarnRatio / CapacityCriticalRatio относительно ExpectedEPSMax.
	CapacityWarnRatio     float64 `json:"capacity_warn_ratio"`
	CapacityCriticalRatio float64 `json:"capacity_critical_ratio"`
}

// DefaultIngestSLO — product defaults (appliance).
func DefaultIngestSLO() IngestSLO {
	return IngestSLO{
		QueueWarnRatio:        0.75,
		QueueCriticalRatio:    0.90,
		BufferWarnLines:       10_000,
		BufferCriticalLines:   100_000,
		DropsWarnPerSec:       0, // any sustained drops/s → warn
		DropsCriticalPerSec:   100,
		LagWarnSec:            60,
		LagCriticalSec:        300,
		CapacityWarnRatio:     1.05,
		CapacityCriticalRatio: 1.25,
	}
}

// Normalize заполняет нулевые/невалидные поля дефолтами.
func (s IngestSLO) Normalize() IngestSLO {
	d := DefaultIngestSLO()
	if s.QueueWarnRatio <= 0 || s.QueueWarnRatio >= 1 {
		s.QueueWarnRatio = d.QueueWarnRatio
	}
	if s.QueueCriticalRatio <= s.QueueWarnRatio || s.QueueCriticalRatio > 1 {
		s.QueueCriticalRatio = d.QueueCriticalRatio
	}
	if s.BufferWarnLines <= 0 {
		s.BufferWarnLines = d.BufferWarnLines
	}
	if s.BufferCriticalLines <= s.BufferWarnLines {
		s.BufferCriticalLines = d.BufferCriticalLines
	}
	if s.DropsCriticalPerSec <= 0 {
		s.DropsCriticalPerSec = d.DropsCriticalPerSec
	}
	if s.DropsWarnPerSec < 0 {
		s.DropsWarnPerSec = d.DropsWarnPerSec
	}
	if s.LagWarnSec <= 0 {
		s.LagWarnSec = d.LagWarnSec
	}
	if s.LagCriticalSec <= s.LagWarnSec {
		s.LagCriticalSec = d.LagCriticalSec
	}
	if s.CapacityWarnRatio <= 1 {
		s.CapacityWarnRatio = d.CapacityWarnRatio
	}
	if s.CapacityCriticalRatio <= s.CapacityWarnRatio {
		s.CapacityCriticalRatio = d.CapacityCriticalRatio
	}
	return s
}
