package backupstore

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestIsSafeIdentAndMissing(t *testing.T) {
	if isSafeIdent("") || isSafeIdent("a-b") || isSafeIdent("a;b") {
		t.Fatal("unsafe")
	}
	if !isSafeIdent("traffic_logs") || !isSafeIdent("A1_") {
		t.Fatal("safe")
	}
	if isMissingBackupObject(nil) {
		t.Fatal("nil")
	}
	for _, msg := range []string{"NOT FOUND", "doesn't exist", "Does Not Exist", "UNKNOWN_TABLE", "no_such", "cannot find", "backup doesn't", "backup does not"} {
		if !isMissingBackupObject(errors.New(msg)) {
			t.Fatalf("missed %q", msg)
		}
	}
	if isMissingBackupObject(errors.New("permission denied")) {
		t.Fatal("false positive")
	}
}

func TestBackupRunnerNilAndValidation(t *testing.T) {
	var nilR *BackupRunner
	if _, err := nilR.TableExists(context.Background(), "x"); err == nil {
		t.Fatal("nil runner")
	}
	r := NewBackupRunner(nil)
	if _, err := r.TableExists(context.Background(), "x"); err == nil {
		t.Fatal("nil conn")
	}
	if err := r.BackupTables(context.Background(), "n", []string{"t"}); err == nil {
		t.Fatal("nil conn backup")
	}
	if err := r.DropTables(context.Background(), []string{"t"}); err == nil {
		t.Fatal("nil drop")
	}
	if err := r.RestoreTablesAs(context.Background(), "n", [][2]string{{"a", "b"}}); err == nil {
		t.Fatal("nil restore")
	}

	ch := &stubConn{}
	r = NewBackupRunner(ch)
	if err := r.BackupTables(context.Background(), "", []string{"t"}); err == nil {
		t.Fatal("empty name")
	}
	if err := r.BackupTables(context.Background(), "bad'name", []string{"t"}); err == nil {
		t.Fatal("bad name")
	}
	if err := r.BackupTables(context.Background(), "ok", []string{"bad-name"}); err == nil {
		t.Fatal("bad table")
	}
	if err := r.BackupTables(context.Background(), "ok", nil); err == nil {
		t.Fatal("no tables")
	}
	if err := r.DropTables(context.Background(), []string{"bad;"}); err == nil {
		t.Fatal("bad drop")
	}
	if err := r.RestoreTablesAs(context.Background(), "bad/name", [][2]string{{"a", "b"}}); err == nil {
		t.Fatal("bad restore name")
	}
	if err := r.RestoreTablesAs(context.Background(), "ok", [][2]string{{"", "b"}}); err == nil {
		t.Fatal("bad pair")
	}
}

func TestBackupRunnerHappyPaths(t *testing.T) {
	ch := &stubConn{}
	r := NewBackupRunner(ch)
	ch.pushRow(uint64(1))
	ok, err := r.TableExists(context.Background(), "traffic_logs")
	if err != nil || !ok {
		t.Fatalf("exists=%v err=%v", ok, err)
	}
	ch.pushRow(uint64(0))
	ok, err = r.TableExists(context.Background(), "missing")
	if err != nil || ok {
		t.Fatalf("missing exists=%v err=%v", ok, err)
	}
	ch.pushRowErr(errors.New("row fail"))
	if _, err := r.TableExists(context.Background(), "x"); err == nil {
		t.Fatal("want row err")
	}

	if err := r.BackupTables(context.Background(), "ga_bak1", []string{"traffic_logs", " geo_ranges "}); err != nil {
		t.Fatal(err)
	}
	if len(ch.execCalls) != 1 || !strings.Contains(ch.execCalls[0], "BACKUP TABLE traffic_logs, TABLE geo_ranges") {
		t.Fatalf("%v", ch.execCalls)
	}
	ch.queueExecErr(errors.New("exec boom"))
	if err := r.BackupTables(context.Background(), "ga_bak2", []string{"traffic_logs"}); err == nil {
		t.Fatal("want exec err")
	}

	ch.execCalls = nil
	if err := r.DropTables(context.Background(), []string{"ga_bak_traffic_logs"}); err != nil {
		t.Fatal(err)
	}
	if len(ch.execCalls) != 1 || !strings.Contains(ch.execCalls[0], "DROP TABLE IF EXISTS ga_bak_traffic_logs") {
		t.Fatalf("%v", ch.execCalls)
	}
	ch.queueExecErr(errors.New("drop fail"))
	if err := r.DropTables(context.Background(), []string{"x"}); err == nil {
		t.Fatal("want drop err")
	}

	ch.execCalls = nil
	// drop ok, restore missing → skip, second pair restores
	ch.queueExecErr(nil) // drop dest1
	ch.queueExecErr(errors.New("object not found"))
	ch.queueExecErr(nil) // drop dest2
	ch.queueExecErr(nil) // restore dest2
	if err := r.RestoreTablesAs(context.Background(), "snap1", [][2]string{
		{"traffic_logs", "ga_bak_traffic_logs"},
		{"geo_ranges", "ga_bak_geo_ranges"},
	}); err != nil {
		t.Fatal(err)
	}

	ch.execCalls = nil
	ch.queueExecErr(nil)
	ch.queueExecErr(errors.New("not found"))
	if err := r.RestoreTablesAs(context.Background(), "snap2", [][2]string{{"a", "b"}}); err == nil {
		t.Fatal("nothing restored")
	}

	ch.execCalls = nil
	ch.queueExecErr(errors.New("drop fail"))
	if err := r.RestoreTablesAs(context.Background(), "snap3", [][2]string{{"a", "b"}}); err == nil {
		t.Fatal("want drop fail")
	}

	ch.execCalls = nil
	ch.queueExecErr(nil)
	ch.queueExecErr(errors.New("permission denied"))
	if err := r.RestoreTablesAs(context.Background(), "snap4", [][2]string{{"a", "b"}}); err == nil {
		t.Fatal("want hard restore err")
	}

	// Map shadow helpers (exercise wrappers; may fail if CH rejects but stubs succeed)
	ch.execCalls = nil
	if err := r.DropMapShadow(context.Background()); err != nil {
		t.Fatal(err)
	}
	ch.execCalls = nil
	// RestoreMapShadow: for each pair drop+restore; stub succeeds all
	if err := r.RestoreMapShadow(context.Background(), "mapsnap"); err != nil {
		t.Fatal(err)
	}
}
