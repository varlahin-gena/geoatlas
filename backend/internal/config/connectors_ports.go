package config

import (
	"fmt"
	"strconv"
	"strings"

	"geoatlas/internal/usecase/connectors"
)

// ParseConnectorExtraPorts parses "5514-5529" or "5514" into a PortRange.
func ParseConnectorExtraPorts(s string) (connectors.PortRange, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return connectors.PortRange{}, nil
	}
	if strings.Contains(s, ",") {
		return connectors.PortRange{}, fmt.Errorf("GA_CONNECTOR_EXTRA_PORTS: use min-max range, not a list")
	}
	parts := strings.Split(s, "-")
	if len(parts) == 1 {
		p, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || p < 1024 || p > 65535 {
			return connectors.PortRange{}, fmt.Errorf("GA_CONNECTOR_EXTRA_PORTS: invalid port %q", s)
		}
		return connectors.PortRange{Min: p, Max: p}, nil
	}
	if len(parts) != 2 {
		return connectors.PortRange{}, fmt.Errorf("GA_CONNECTOR_EXTRA_PORTS: want min-max, got %q", s)
	}
	lo, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	hi, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || lo < 1024 || hi > 65535 || hi < lo {
		return connectors.PortRange{}, fmt.Errorf("GA_CONNECTOR_EXTRA_PORTS: invalid range %q", s)
	}
	return connectors.PortRange{Min: lo, Max: hi}, nil
}
