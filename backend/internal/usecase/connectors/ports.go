package connectors

import "context"

// Store — JSON persistence for connectors document.
type Store interface {
	Load() (Document, error)
	Save(Document) error
}

// ConfWriter renders syslog-ng fragment for enabled connectors.
type ConfWriter interface {
	Write(doc Document) (path string, err error)
}

// Reloader triggers syslog-ng-ctl reload (HTTP agent in syslog-ng container).
type Reloader interface {
	Reload(ctx context.Context) error
}
