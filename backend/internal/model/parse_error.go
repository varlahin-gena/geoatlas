package model

import "time"

// ParseError — строка лога, которую не удалось распарсить (для вставки в parse_errors).
type ParseError struct {
	Timestamp time.Time
	Vendor    string
	Reason    string
	Raw       string
}

// ParseErrorRow — запись parse_errors для выдачи в API.
type ParseErrorRow struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Vendor    string    `json:"vendor"`
	Reason    string    `json:"reason"`
	Raw       string    `json:"raw"`
}
