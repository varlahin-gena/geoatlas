package sqlclause

import (
	"strings"
	"testing"

	"geoatlas/internal/mapsearch"
)

func TestMapSearchSQLOrNotEmptyAndNodeCols(t *testing.T) {
	c := mapsearch.Compile("country:DE OR action:block")
	clause, args := MapSearchSQL(c, LogsMapSearchColumns)
	if !strings.Contains(clause, " OR ") || len(args) < 2 {
		t.Fatalf("%s %v", clause, args)
	}
	c2 := mapsearch.Compile("NOT action:allow")
	clause2, _ := MapSearchSQL(c2, LogsMapSearchColumns)
	if !strings.Contains(clause2, "NOT") {
		t.Fatalf("%s", clause2)
	}
	empty := mapsearch.Compile("")
	clause3, args3 := MapSearchSQL(empty, LogsMapSearchColumns)
	if clause3 != "" || len(args3) != 0 {
		t.Fatalf("%q %v", clause3, args3)
	}

	c4 := mapsearch.Compile("country:DE")
	clause4, _ := MapSearchSQL(c4, GeoMapSearchColumns)
	if clause4 == "" {
		t.Fatal("geo clause empty")
	}
	c5 := mapsearch.Compile("ip:1.2.3.4")
	clause5, _ := MapSearchSQL(c5, IPAggMapSearchColumns)
	if clause5 == "" {
		t.Fatal("ipagg clause")
	}
}
