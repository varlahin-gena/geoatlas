package migrate

import (
	"context"
	"strings"
	"testing"
)

func TestReplaceMaterializedViewRejectsEmpty(t *testing.T) {
	err := replaceMaterializedView(context.TODO(), nil, "", func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err=%v", err)
	}
	err = replaceMaterializedView(context.TODO(), nil, "mv", nil)
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("err=%v", err)
	}
}
