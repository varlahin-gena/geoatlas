package connectors

import (
	"fmt"
	"regexp"
	"strings"
)

var idRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func normalizeProtocols(p string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case ProtoUDP, "u":
		return ProtoUDP, nil
	case ProtoTCP, "t":
		return ProtoTCP, nil
	case ProtoBoth, "udp+tcp", "tcp+udp", "all", "":
		return ProtoBoth, nil
	default:
		return "", fmt.Errorf("%w: protocols must be udp|tcp|both", ErrInvalid)
	}
}

func normalizeID(id string) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !idRe.MatchString(id) {
		return "", fmt.Errorf("%w: id must match %s", ErrInvalid, idRe.String())
	}
	return id, nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if len(name) > 128 {
		return "", fmt.Errorf("%w: name too long", ErrInvalid)
	}
	return name, nil
}

func validatePort(port int, range_ PortRange, id string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%w: port out of range", ErrInvalid)
	}
	if port < 1024 && port != DefaultPort {
		return fmt.Errorf("%w: privileged ports other than 514 are not allowed", ErrInvalid)
	}
	if id == DefaultID {
		if port != DefaultPort {
			return fmt.Errorf("%w: builtin connector must use port 514", ErrInvalid)
		}
		return nil
	}
	if !range_.Allows(port) {
		return ErrPortNotAllowed
	}
	return nil
}
