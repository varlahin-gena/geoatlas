package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestServiceUpdateAndStatus(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := selfSignedPair(t, "geoatlas.local")

	svc := New(Config{
		CertDir:      dir,
		CertFile:     "fullchain.pem",
		KeyFile:      "privkey.pem",
		HTTPSEnabled: "auto",
		HTTPSPort:    "443",
		HTTPRedirect: "1",
	})

	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Configured != true || st.Writable != true {
		t.Fatalf("expected configured writable dir, got %+v", st)
	}

	if err := svc.Update(UpdateInput{CertPEM: certPEM, KeyPEM: keyPEM}); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(dir, "fullchain.pem")) {
		t.Fatal("cert not written")
	}

	st, err = svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.CertPresent || !st.KeyPresent || st.Cert == nil {
		t.Fatalf("expected cert info, got %+v", st)
	}
	if st.Cert.Subject == "" || st.Cert.DaysLeft < 0 {
		t.Fatalf("unexpected cert info %+v", st.Cert)
	}
}

func TestServiceRejectsMismatchedKey(t *testing.T) {
	dir := t.TempDir()
	certPEM, _ := selfSignedPair(t, "a.example")
	_, keyPEM := selfSignedPair(t, "b.example")
	svc := New(Config{CertDir: dir, CertFile: "fullchain.pem", KeyFile: "privkey.pem"})
	if err := svc.Update(UpdateInput{CertPEM: certPEM, KeyPEM: keyPEM}); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func selfSignedPair(t *testing.T, cn string) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{cn},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return string(certPEM), string(keyPEM)
}

func TestDirWritableUsesTempDir(t *testing.T) {
	dir := t.TempDir()
	if !dirWritable(dir) {
		t.Fatal("temp dir should be writable")
	}
	if runtime.GOOS == "windows" {
		t.Skip("chmod read-only is unreliable on Windows")
	}
	_ = os.Chmod(dir, 0o555)
	if dirWritable(dir) {
		t.Fatal("read-only dir should not be writable")
	}
}
