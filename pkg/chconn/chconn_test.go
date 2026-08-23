package chconn_test

import (
	"testing"

	"geoatlas/pkg/chconn"
)

func TestAuthNormalized(t *testing.T) {
	a := chconn.Auth{}.Normalized()
	if a.Database != "default" || a.Username != "default" {
		t.Fatalf("got %+v", a)
	}
}
