module network_monitor

go 1.25.0

require (
	github.com/ClickHouse/clickhouse-go/v2 v2.17.1
	github.com/prometheus/client_golang v1.22.0
	golang.org/x/crypto v0.31.0
	golang.org/x/sys v0.47.0
	gopkg.in/yaml.v3 v3.0.1
	network_monitor/pkg/chconn v0.0.0
	network_monitor/pkg/syslogngstats v0.0.0
)

replace network_monitor/pkg/chconn => ../pkg/chconn

replace network_monitor/pkg/syslogngstats => ../pkg/syslogngstats

require (
	github.com/ClickHouse/ch-go v0.58.2 // indirect
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.6.1 // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/paulmach/orb v0.10.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	go.opentelemetry.io/otel v1.19.0 // indirect
	go.opentelemetry.io/otel/trace v1.19.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
)
