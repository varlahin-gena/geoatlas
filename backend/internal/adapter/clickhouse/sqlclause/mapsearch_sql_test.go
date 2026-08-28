package sqlclause

import (
	"strings"
	"testing"

	"geoatlas/internal/mapsearch"
)

func TestMapSearchSQLAdvancedAND(t *testing.T) {
	c := mapsearch.Compile("country:Germany AND action:block")
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

func TestMapSearchSQLSrcDstFields(t *testing.T) {
	c := mapsearch.Compile("src_ip=10.0.0.1 AND dst_port=443")
	clause, args := MapSearchSQL(c, LogsMapSearchColumns)
	if !strings.Contains(clause, "toString(src_ip)") || !strings.Contains(clause, "toString(dst_port)") {
		t.Fatalf("clause=%s", clause)
	}
	if !strings.Contains(clause, "lowerUTF8") {
		t.Fatalf("expected eq SQL, clause=%s", clause)
	}
	if len(args) < 2 {
		t.Fatalf("args=%v", args)
	}
}

func TestMapSearchSQLActionField(t *testing.T) {
	c := mapsearch.Compile("action=allow")
	clause, args := MapSearchSQL(c, LogsMapSearchColumns)
	if !strings.Contains(clause, "action") || len(args) != 1 || args[0] != "allow" {
		t.Fatalf("clause=%s args=%v", clause, args)
	}
	c2 := mapsearch.Compile("rule:block")
	clause2, _ := MapSearchSQL(c2, LogsMapSearchColumns)
	if !strings.Contains(clause2, "action") {
		t.Fatalf("legacy rule alias should map to action, clause=%s", clause2)
	}
}
func TestMapSearchSQLNeJoinsWithAnd(t *testing.T) {
	c := mapsearch.Compile("ip!=10.0.0.1")
	clause, _ := MapSearchSQL(c, LogsMapSearchColumns)
	if !strings.Contains(clause, " AND ") {
		t.Fatalf("expected AND join for ne, clause=%s", clause)
	}
}
