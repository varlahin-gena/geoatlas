package reputationjob

import "testing"

func TestLooksDeprecatedEmpty(t *testing.T) {
	if looksDeprecatedEmpty([]byte("# spamhaus drop\n1.2.3.0/24\n")) {
		t.Fatal("active feed with IPs must not look deprecated")
	}
	if !looksDeprecatedEmpty([]byte("# DEPRECATED — use new URL\n# no ips left\n")) {
		t.Fatal("deprecated empty feed")
	}
	if looksDeprecatedEmpty([]byte("# DEPRECATED\n8.8.8.8\n")) {
		t.Fatal("deprecated but still has IP")
	}
	if looksDeprecatedEmpty([]byte("just a normal list\n")) {
		t.Fatal("no deprecated marker")
	}
}

func TestContainsASCIIFold(t *testing.T) {
	if !containsASCIIFold([]byte("Foo DEPRECATED bar"), "deprecated") {
		t.Fatal("case-insensitive hit")
	}
	if containsASCIIFold([]byte("deprecat"), "deprecated") {
		t.Fatal("partial must miss")
	}
}
