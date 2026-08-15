package mapsearch

import "testing"

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
}

func TestCompileInvalidIsEmpty(t *testing.T) {
	c := Compile(`country:"unterminated`)
	if !c.Empty {
		t.Fatalf("want empty on error, got %+v", c)
	}
}
