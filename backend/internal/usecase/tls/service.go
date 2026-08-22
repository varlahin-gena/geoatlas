package tls

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrUnavailable = errors.New("tls cert storage unavailable")
	ErrInvalidPEM  = errors.New("invalid pem")
)

type Config struct {
	CertDir      string
	CertFile     string
	KeyFile      string
	HTTPSEnabled string
	HTTPSPort    string
	HTTPRedirect string
	ReloadCmd    string
}

type CertInfo struct {
	Subject           string   `json:"subject,omitempty"`
	Issuer            string   `json:"issuer,omitempty"`
	NotBefore         string   `json:"not_before,omitempty"`
	NotAfter          string   `json:"not_after,omitempty"`
	DaysLeft          int      `json:"days_left,omitempty"`
	SANs              []string `json:"sans,omitempty"`
	FingerprintSHA256 string   `json:"fingerprint_sha256,omitempty"`
	SelfSigned        bool     `json:"self_signed,omitempty"`
}

type Status struct {
	Configured   bool     `json:"configured"`
	Writable     bool     `json:"writable"`
	CertPresent  bool     `json:"cert_present"`
	KeyPresent   bool     `json:"key_present"`
	HTTPSEnabled string   `json:"https_enabled"`
	HTTPSPort    string   `json:"https_port"`
	HTTPRedirect string   `json:"http_redirect"`
	CertPath     string   `json:"cert_path,omitempty"`
	KeyPath      string   `json:"key_path,omitempty"`
	Cert         *CertInfo `json:"cert,omitempty"`
}

type UpdateInput struct {
	CertPEM string
	KeyPEM  string
}

type ReloadResult struct {
	Reloaded        bool   `json:"reloaded"`
	RestartRequired bool   `json:"restart_required"`
	Message         string `json:"message,omitempty"`
}

type Service struct {
	cfg Config
}

func New(cfg Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) available() bool {
	return strings.TrimSpace(s.cfg.CertDir) != ""
}

func (s *Service) certPath() string {
	return filepath.Join(s.cfg.CertDir, s.cfg.CertFile)
}

func (s *Service) keyPath() string {
	return filepath.Join(s.cfg.CertDir, s.cfg.KeyFile)
}

func (s *Service) Status() (Status, error) {
	out := Status{
		HTTPSEnabled: defaultStr(s.cfg.HTTPSEnabled, "auto"),
		HTTPSPort:    defaultStr(s.cfg.HTTPSPort, "443"),
		HTTPRedirect: defaultStr(s.cfg.HTTPRedirect, "1"),
	}
	if !s.available() {
		return out, nil
	}
	out.Configured = true
	out.CertPath = s.certPath()
	out.KeyPath = s.keyPath()

	if err := os.MkdirAll(s.cfg.CertDir, 0o755); err == nil {
		out.Writable = dirWritable(s.cfg.CertDir)
	}

	certPEM, certErr := os.ReadFile(s.certPath())
	keyPresent := fileExists(s.keyPath())
	out.KeyPresent = keyPresent
	if certErr == nil && len(certPEM) > 0 {
		out.CertPresent = true
		if info, err := parseCertInfo(certPEM); err == nil {
			out.Cert = &info
		}
	}
	return out, nil
}

func (s *Service) Update(in UpdateInput) error {
	if !s.available() {
		return ErrUnavailable
	}
	certPEM := strings.TrimSpace(in.CertPEM)
	keyPEM := strings.TrimSpace(in.KeyPEM)
	if certPEM == "" || keyPEM == "" {
		return fmt.Errorf("%w: cert_pem and key_pem are required", ErrInvalidPEM)
	}
	if err := validatePair(certPEM, keyPEM); err != nil {
		return err
	}
	if err := os.MkdirAll(s.cfg.CertDir, 0o755); err != nil {
		return err
	}
	if !dirWritable(s.cfg.CertDir) {
		return ErrUnavailable
	}
	if err := writeAtomic(s.certPath(), []byte(normalizePEM(certPEM)+"\n"), 0o644); err != nil {
		return err
	}
	return writeAtomic(s.keyPath(), []byte(normalizePEM(keyPEM)+"\n"), 0o600)
}

func (s *Service) Reload() ReloadResult {
	if cmd := strings.TrimSpace(s.cfg.ReloadCmd); cmd != "" {
		c := exec.Command("sh", "-c", cmd)
		if err := c.Run(); err != nil {
			return ReloadResult{
				RestartRequired: true,
				Message:         "Сертификаты сохранены, но команда перезагрузки не выполнилась. Перезапустите frontend вручную.",
			}
		}
		return ReloadResult{
			Reloaded: true,
			Message:  "Сертификаты применены.",
		}
	}
	return ReloadResult{
		RestartRequired: true,
		Message:         "Сертификаты сохранены. Перезапустите frontend (./start.sh или docker compose restart frontend), чтобы nginx подхватил HTTPS.",
	}
}

func validatePair(certPEM, keyPEM string) error {
	cert, err := parseCertificateBlock(certPEM)
	if err != nil {
		return fmt.Errorf("%w: certificate: %v", ErrInvalidPEM, err)
	}
	key, err := parsePrivateKeyBlock(keyPEM)
	if err != nil {
		return fmt.Errorf("%w: private key: %v", ErrInvalidPEM, err)
	}
	if err := keyMatchesCert(cert, key); err != nil {
		return err
	}
	now := time.Now()
	if now.After(cert.NotAfter) {
		return fmt.Errorf("%w: certificate already expired", ErrInvalidPEM)
	}
	return nil
}

func parseCertInfo(certPEM []byte) (CertInfo, error) {
	cert, err := parseCertificateBlock(string(certPEM))
	if err != nil {
		return CertInfo{}, err
	}
	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}
	sans := append([]string{}, cert.DNSNames...)
	sans = append(sans, cert.EmailAddresses...)
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}
	sum := sha256.Sum256(cert.Raw)
	return CertInfo{
		Subject:           cert.Subject.String(),
		Issuer:            cert.Issuer.String(),
		NotBefore:         cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:          cert.NotAfter.UTC().Format(time.RFC3339),
		DaysLeft:          daysLeft,
		SANs:              sans,
		FingerprintSHA256: fmt.Sprintf("%X", sum),
		SelfSigned:        cert.Subject.String() == cert.Issuer.String(),
	}, nil
}

func parseCertificateBlock(pemText string) (*x509.Certificate, error) {
	var cert *x509.Certificate
	rest := []byte(pemText)
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		parsed, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		cert = parsed
	}
	if cert == nil {
		return nil, errors.New("no certificate block")
	}
	return cert, nil
}

func parsePrivateKeyBlock(pemText string) (any, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("no pem block")
	}
	switch block.Type {
	case "PRIVATE KEY":
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported key type %q", block.Type)
	}
}

func keyMatchesCert(cert *x509.Certificate, key any) error {
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPEM, err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return fmt.Errorf("%w: private key does not match certificate", ErrInvalidPEM)
	}
	return nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tls-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func normalizePEM(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func dirWritable(dir string) bool {
 probe := filepath.Join(dir, ".write-probe")
	if err := os.WriteFile(probe, []byte("1"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(probe)
	return true
}

func defaultStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}
