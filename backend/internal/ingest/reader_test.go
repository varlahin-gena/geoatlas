package ingest

import (
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestFrameReaderOctetCounting(t *testing.T) {
	payload := `<134>1 host msg`
	framed := strconv.Itoa(len(payload)) + " " + payload + "\n"
	fr := newFrameReader(strings.NewReader(framed))
	got, err := fr.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if got != payload {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

func TestFrameReaderIPLineNotSplit(t *testing.T) {
	// Starts with digits (IP) — must not be treated as octet-counting.
	line := "10.0.0.1 dst=8.8.8.8 action=allow\n"
	fr := newFrameReader(strings.NewReader(line + "second line\n"))
	got, err := fr.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimRight(line, "\n")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	got2, err := fr.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if got2 != "second line" {
		t.Fatalf("second = %q", got2)
	}
}

func TestFrameReaderEOF(t *testing.T) {
	fr := newFrameReader(strings.NewReader(""))
	_, err := fr.ReadLine()
	if err != io.EOF {
		t.Fatalf("err = %v, want EOF", err)
	}
}

func TestFrameReaderLFTooLarge(t *testing.T) {
	// No newline until past maxFrameBytes — must reject, not grow unboundedly.
	huge := strings.Repeat("a", maxFrameBytes+64) + "\nnext\n"
	fr := newFrameReader(strings.NewReader(huge))
	_, err := fr.ReadLine()
	if err != errFrameTooLarge {
		t.Fatalf("err = %v, want %v", err, errFrameTooLarge)
	}
	got, err := fr.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if got != "next" {
		t.Fatalf("after oversize got %q, want next", got)
	}
}

func TestFrameReaderOctetCountingMax(t *testing.T) {
	payload := strings.Repeat("x", 100)
	framed := strconv.Itoa(len(payload)) + " " + payload
	fr := newFrameReader(strings.NewReader(framed))
	got, err := fr.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if got != payload {
		t.Fatalf("len=%d want %d", len(got), len(payload))
	}
}

func TestFrameReaderRejectsHugeLengthField(t *testing.T) {
	// Digits without space beyond maxLenDigits → fall back to LF.
	digits := strings.Repeat("9", 20) + " rest\n"
	fr := newFrameReader(strings.NewReader(digits))
	got, err := fr.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimRight(digits, "\n")
	if got != want {
		t.Fatalf("got %q", got)
	}
}
