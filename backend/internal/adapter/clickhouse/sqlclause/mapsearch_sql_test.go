package sqlclause

import (
	"strings"
	"testing"

	"geoatlas/internal/mapsearch"
)

func TestMapSearchSQLAdvancedAND(t *testing.T) {
	c := mapsearch.Compile("country:Germany AND rule:block")
	clause, args := MapSearchSQL(c, LogsMapSearchColumns)
	if !strings.Contains(clause, "AND") || len(args) < 2 {
		t.Fatalf("clause=%s args=%v", clause, args)
	}
	for _, a := range args {
		if s, ok := a.(string); ok && strings.Contains(clause, s) {
			t.Fatal("value interpolated")
		}
	}
}

func TestMapSearchSQLSimpleBind(t *testing.T) {
	c := mapsearch.Compile("Moscow")
	clause, args := MapSearchSQL(c, LogsMapSearchColumns)
	if !strings.Contains(clause, "concat_ws") || len(args) != 1 || args[0] != "Moscow" {
		t.Fatalf("clause=%s args=%v", clause, args)
	}
}

func TestMapSearchSQLCountryAliases(t *testing.T) {
	c := mapsearch.Compile("country:Россия")
	clause, args := MapSearchSQL(c, LogsMapSearchColumns)
	if !strings.Contains(clause, "src_country") {
		t.Fatalf("clause=%s", clause)
	}
	found := false
	for _, a := range args {
		if a == "Russia" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args=%v", args)
	}
}
