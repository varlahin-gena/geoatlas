// Package threatprot — Apigee-style API threat controls for the Go backend
// (SpikeArrest, JSON threat protection, injection guard, response headers).
package threatprot

// JSON structure limits (Apigee JSONThreatProtection defaults, tuned for GeoAtlas APIs).
const (
	MaxJSONContainerDepth = 5
	MaxJSONStringLength   = 500
)
