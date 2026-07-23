package storage

import (
	"network_monitor/internal/storage/aggstate"
	"network_monitor/internal/storage/migrate"
	"network_monitor/internal/storage/query"
)

// --- aggstate ---

type EdgesAggStatus = aggstate.EdgesAggStatus

var (
	GetEdgesAggStatus   = aggstate.GetEdgesAggStatus
	SetEdgesAggStatus   = aggstate.SetEdgesAggStatus
	PreferDailyEdgesAgg = aggstate.PreferDailyEdgesAgg
	PreferGeoEdgesAgg   = aggstate.PreferGeoEdgesAgg
)

// --- migrate ---

var (
	EnsureEdgesAggSchema       = migrate.EnsureEdgesAggSchema
	EnsureEdgesAgg             = migrate.EnsureEdgesAgg
	BackfillEdgesAgg           = migrate.BackfillEdgesAgg
	RefreshEdgesAggReady       = migrate.RefreshEdgesAggReady
	EnsureGeoEdgesAggSchema    = migrate.EnsureGeoEdgesAggSchema
	EnsureGeoEdgesAgg          = migrate.EnsureGeoEdgesAgg
	BackfillGeoEdgesAgg        = migrate.BackfillGeoEdgesAgg
	RefreshGeoEdgesAggReady    = migrate.RefreshGeoEdgesAggReady
	RebuildGeoEdgesLookback    = migrate.RebuildGeoEdgesLookback
	EnsureTrafficLogsSuccess   = migrate.EnsureTrafficLogsSuccess
	EnsureTTLOnlyDropParts     = migrate.EnsureTTLOnlyDropParts
)

// --- query ---

var (
	ConfigureQuerySettings        = query.ConfigureQuerySettings
	ScanRawAggs                   = query.ScanRawAggs
	ScanRawAggsForTimeRange       = query.ScanRawAggsForTimeRange
	ScanGeoEdgesForTimeRange      = query.ScanGeoEdgesForTimeRange
	ScanGeoMissingIPsForTimeRange = query.ScanGeoMissingIPsForTimeRange
)
