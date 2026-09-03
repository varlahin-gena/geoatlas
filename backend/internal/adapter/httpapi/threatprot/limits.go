// Package threatprot — Apigee-style API threat controls for the Go backend
// (SpikeArrest, JSON threat protection, injection guard, response headers).
package threatprot

// JSON structure limits (Apigee JSONThreatProtection defaults, tuned for GeoAtlas APIs).
const (
	MaxJSONContainerDepth = 5
	// MaxJSONObjectEntryNameLength — Apigee ObjectEntryNameLength (50), slight headroom.
	MaxJSONObjectEntryNameLength = 64
	// MaxJSONObjectEntryCount — Apigee ObjectEntryCount (25); raised for sparse DTOs.
	MaxJSONObjectEntryCount = 64
	// MaxJSONArrayElementCount — Apigee ArrayElementCount (100); batch deletes need headroom.
	MaxJSONArrayElementCount = 256
	// MaxJSONStringLength covers typical mutation fields (URLs ≤2048, notes, names).
	MaxJSONStringLength = 8192
	// MaxJSONStringLengthLarge covers TLS PEM upload (OpenAPI maxLength 65536).
	MaxJSONStringLengthLarge = 65536
)
