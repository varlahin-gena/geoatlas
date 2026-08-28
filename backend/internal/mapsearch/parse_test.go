package mapsearch

import "testing"

func TestCompileSimple(t *testing.T) {
	c := Compile("  tcp  ")
	if c.Empty || c.Simple != "tcp" || c.Root != nil {
		t.Fatalf("%+v", c)
	}
}

func TestCompileAdvancedAND(t *testing.T) {
	c := Compile("country:Germany AND action:block")
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

func TestCompileAttackerTargetFields(t *testing.T) {
	c := Compile("src_ip=10.0.0.1 AND dst_port=443")
	if c.Empty || c.Simple != "" || c.Root == nil || c.Root.Kind != KindAnd {
		t.Fatalf("%+v", c)
	}
	c2 := Compile("attacker:1.2.3.4")
	if c2.Empty || c2.Root == nil || c2.Root.Field != FieldSrcIP || c2.Root.Op != OpContains {
		t.Fatalf("%+v", c2)
	}
	c3 := Compile("src_ip!=10.0.0.1")
	if c3.Empty || c3.Root == nil || c3.Root.Op != OpNe {
		t.Fatalf("%+v", c3)
	}
}
