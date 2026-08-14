package mapsearch

import (
	"strings"
	"testing"
)

func TestCompileSimple(t *testing.T) {
	c := Compile("  tcp  ")
	if c.Empty || c.Simple != "tcp" || c.Root != nil {
		t.Fatalf("%+v", c)
	}
}

func TestCompileAdvancedAND(t *testing.T) {
	c := Compile("country:Germany AND rule:block")
	if c.Empty || c.Simple != "" || c.Root == nil || c.Root.Kind != KindAnd {
		t.Fatalf("%+v", c)
	}
	clause, args := c.SQL(LogsColumns)
	if !strings.Contains(clause, "AND") || len(args) < 2 {
		t.Fatalf("clause=%s args=%v", clause, args)
	}
	for _, a := range args {
		if s, ok := a.(string); ok && strings.Contains(clause, s) {
			t.Fatal("value interpolated")
		}
	}
}

func TestCompileInvalidIsEmpty(t *testing.T) {
	c := Compile(`country:"unterminated`)
	if !c.Empty {
		t.Fatalf("want empty on error, got %+v", c)
	}
}

func TestSQLSimpleBind(t *testing.T) {
	c := Compile("Moscow")
	clause, args := c.SQL(LogsColumns)
	if !strings.Contains(clause, "concat_ws") || len(args) != 1 || args[0] != "Moscow" {
		t.Fatalf("clause=%s args=%v", clause, args)
	}
}

func TestCountryAliases(t *testing.T) {
	c := Compile("country:Россия")
	clause, args := c.SQL(LogsColumns)
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
