package loginthrottle

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func swapResolver(t *testing.T, fn func(string) ([]string, error)) {
	t.Helper()
	prev := lookupHost
	lookupHost = fn
	t.Cleanup(func() { lookupHost = prev })
}

func swapTTL(t *testing.T, d time.Duration) {
	t.Helper()
	prev := trustedHostTTL
	trustedHostTTL = d
	t.Cleanup(func() { trustedHostTTL = prev })
}

// Следующий тест пересоберёт снапшот сам; ресолвер к тому моменту уже настоящий.
func dropTrusted(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { trusted.Store(nil) })
}

// Главное свойство: путь обработки запроса не ходит в DNS.
func TestIsTrustedProxyDoesNotResolvePerCall(t *testing.T) {
	var calls atomic.Int32
	swapResolver(t, func(string) ([]string, error) {
		calls.Add(1)
		return []string{"10.9.9.9"}, nil
	})
	swapTTL(t, time.Hour)
	dropTrusted(t)

	ConfigureTrustedProxies([]string{"frontend"})
	if got := calls.Load(); got != 1 {
		t.Fatalf("Configure resolved %d times, want exactly 1", got)
	}

	for range 1000 {
		if !isTrustedProxy("10.9.9.9") {
			t.Fatal("resolved proxy address must be trusted")
		}
		if isTrustedProxy("203.0.113.1") {
			t.Fatal("unknown address must not be trusted")
		}
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("hot path performed %d extra DNS lookups", got-1)
	}
}

func TestTrustedProxyRefreshesAfterTTL(t *testing.T) {
	var addrs atomic.Value
	addrs.Store([]string{"10.9.9.9"})
	swapResolver(t, func(string) ([]string, error) {
		return addrs.Load().([]string), nil
	})
	swapTTL(t, 10*time.Millisecond)
	dropTrusted(t)

	ConfigureTrustedProxies([]string{"frontend"})
	if !isTrustedProxy("10.9.9.9") {
		t.Fatal("initial address must be trusted")
	}

	addrs.Store([]string{"10.9.9.10"})
	time.Sleep(20 * time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if isTrustedProxy("10.9.9.10") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("snapshot was not refreshed after TTL expiry")
}

// Сбой резолвера не должен снимать доверие к прокси: иначе при
// GA_REQUIRE_PROXY=1 весь API начнёт отвечать 403.
func TestTrustedProxyKeepsLastGoodOnResolverFailure(t *testing.T) {
	var down atomic.Bool
	swapResolver(t, func(string) ([]string, error) {
		if down.Load() {
			return nil, errors.New("dns unavailable")
		}
		return []string{"10.9.9.9"}, nil
	})
	swapTTL(t, time.Hour)
	dropTrusted(t)

	ConfigureTrustedProxies([]string{"frontend"})
	down.Store(true)
	refreshTrustedHosts() // то же, что сделала бы фоновая горутина

	if !isTrustedProxy("10.9.9.9") {
		t.Fatal("DNS outage must not drop the last known proxy address")
	}
}

func TestLoopbackTrustedBeforeConfigure(t *testing.T) {
	prev := trusted.Load()
	trusted.Store(nil)
	t.Cleanup(func() { trusted.Store(prev) })

	if !isTrustedProxy("127.0.0.1") {
		t.Fatal("loopback must be trusted without explicit configuration")
	}
	if !isTrustedProxy("::1") {
		t.Fatal("IPv6 loopback must be trusted without explicit configuration")
	}
	if isTrustedProxy("203.0.113.1") {
		t.Fatal("public address must not be trusted by default")
	}
}

func BenchmarkIsTrustedProxy(b *testing.B) {
	prevLookup, prevTTL := lookupHost, trustedHostTTL
	lookupHost = func(string) ([]string, error) { return []string{"10.9.9.9"}, nil }
	trustedHostTTL = time.Hour
	b.Cleanup(func() {
		lookupHost, trustedHostTTL = prevLookup, prevTTL
		trusted.Store(nil)
	})
	ConfigureTrustedProxies([]string{"frontend", "10.0.0.0/8"})

	b.ReportAllocs()
	for b.Loop() {
		_ = isTrustedProxy("10.9.9.9")
	}
}
