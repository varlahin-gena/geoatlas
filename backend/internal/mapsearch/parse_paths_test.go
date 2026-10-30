package mapsearch

import "testing"

func TestCompileORNotParenAndError(t *testing.T) {
	c := Compile("country:DE OR action:block")
	if c.Empty || c.Root == nil || c.Root.Kind != KindOr {
		t.Fatalf("%+v", c)
	}
	c2 := Compile("NOT action:allow")
	if c2.Empty || c2.Root == nil || c2.Root.Kind != KindNot {
		t.Fatalf("%+v", c2)
	}
	c3 := Compile("(country:DE AND city:Berlin) OR action:deny")
	if c3.Empty || c3.Root == nil {
		t.Fatalf("%+v", c3)
	}
	c4 := Compile(`proto:"tcp"`)
	if c4.Empty {
		t.Fatalf("%+v", c4)
	}
	c5 := Compile("AND")
	if !c5.Empty {
		t.Fatalf("%+v", c5)
	}
	c6 := Compile("country:")
	if !c6.Empty {
		t.Fatalf("%+v", c6)
	}
	err := errString("x")
	if err.Error() != "x" {
		t.Fatal(err.Error())
	}
}
