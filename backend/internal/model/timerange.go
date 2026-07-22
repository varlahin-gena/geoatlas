package model

import "time"

// TimeRange описывает окно выборки для карты и geo-отчётов.
type TimeRange struct {
	Mode   string // minutes, hours, days, absolute
	Amount int
	From   time.Time
	To     time.Time
}
