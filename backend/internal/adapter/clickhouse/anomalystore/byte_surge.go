package anomalystore

import (
	"fmt"

	"geoatlas/internal/adapter/clickhouse/query"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

func byteSurgeArgs(absMin, floor uint64, nets []usecaseanomaly.IPRange) []any {
	_, srcArgs := colInNetsSQL("src_ip", nets)
	_, dstArgs := colInNetsSQL("dst_ip", nets)
	args := make([]any, 0, len(srcArgs)+len(dstArgs)+4)
	args = append(args, srcArgs...)
	args = append(args, absMin, floor)
	args = append(args, dstArgs...)
	args = append(args, absMin, floor)
	return args
}

func byteSurgeSQL(hourly bool, sec int, nets []usecaseanomaly.IPRange) string {
	src, _ := colInNetsSQL("src_ip", nets)
	dst, _ := colInNetsSQL("dst_ip", nets)
	out := byteSurgeSideSQL(hourly, sec, "src_ip", "dst_ip", "out", src)
	in := byteSurgeSideSQL(hourly, sec, "dst_ip", "src_ip", "in", dst)
	return fmt.Sprintf(`
		SELECT host, peer, bytes_now, bytes_prev, events, direction
		FROM (
			%s
			UNION ALL
			%s
		)
		ORDER BY bytes_now DESC
		LIMIT 20
		%s
	`, out, in, query.AggSettings())
}

func byteSurgeSideSQL(hourly bool, sec int, hostCol, peerCol, direction, netClause string) string {
	table := "traffic_logs"
	timeCol := "timestamp"
	eventExpr := "1"
	if hourly {
		table = "traffic_edges_hourly"
		timeCol = "hour"
		eventExpr = "cnt"
	}
	byteExpr := "bytes_sent + bytes_recv"
	return fmt.Sprintf(`
		SELECT
			host,
			argMax(peer, pair_now) AS peer,
			sum(pair_now) AS bytes_now,
			sum(pair_prev) AS bytes_prev,
			sum(pair_events) AS events,
			'%s' AS direction
		FROM (
			SELECT
				toString(%s) AS host,
				toString(%s) AS peer,
				sumIf(%s, %s >= now() - INTERVAL %d SECOND AND %s < now()) AS pair_now,
				sumIf(%s, %s >= now() - INTERVAL %d SECOND AND %s < now() - INTERVAL %d SECOND) AS pair_prev,
				sumIf(%s, %s >= now() - INTERVAL %d SECOND AND %s < now()) AS pair_events
			FROM %s
			WHERE %s >= now() - INTERVAL %d SECOND
			  %s
			GROUP BY %s, %s
		)
		GROUP BY host
		HAVING bytes_now >= ? AND bytes_prev >= ?
	`, direction, hostCol, peerCol,
		byteExpr, timeCol, sec, timeCol,
		byteExpr, timeCol, 2*sec, timeCol, sec,
		eventExpr, timeCol, sec, timeCol,
		table, timeCol, 2*sec, netClause,
		hostCol, peerCol)
}
