package backupfs

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"network_monitor/internal/usecase/backup"
)

var nameRe = regexp.MustCompile(`^nm-\d{8}T\d{6}(Z|[+-]\d{4})$`)

const attachedFile = ".nm-attached"

// DirStore — каталог бэкапов на смонтированном томе clickhouse-backups.
type DirStore struct {
	Root string
}

func New(root string) *DirStore {
	return &DirStore{Root: strings.TrimSpace(root)}
}

func (d *DirStore) DirReady() bool {
	if d == nil || d.Root == "" {
		return false
	}
	st, err := os.Stat(d.Root)
	return err == nil && st.IsDir()
}

func (d *DirStore) List() ([]backup.Entry, error) {
	if !d.DirReady() {
		return nil, nil
	}
	entries, err := os.ReadDir(d.Root)
	if err != nil {
		return nil, err
	}
	var out []backup.Entry
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !nameRe.MatchString(name) {
			continue
		}
		full := filepath.Join(d.Root, name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		size, _ := dirSize(full)
		auth := false
		if _, err := os.Stat(filepath.Join(d.Root, name+".auth.tgz")); err == nil {
			auth = true
			if st, err := os.Stat(filepath.Join(d.Root, name+".auth.tgz")); err == nil {
				size += st.Size()
			}
		}
		created := info.ModTime().UTC()
		if t, ok := parseBackupNameTime(name); ok {
			created = t
		}
		src := ""
		if b, err := os.ReadFile(filepath.Join(d.Root, name+".source")); err == nil {
			src = backup.NormalizeSource(string(b))
		}
		out = append(out, backup.Entry{
			Name:      name,
			CreatedAt: created,
			SizeBytes: size,
			HasAuth:   auth,
			Source:    src,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func parseBackupNameTime(name string) (time.Time, bool) {
	// nm-20060102T150405Z  или  nm-20060102T150405+0300
	if !strings.HasPrefix(name, "nm-") {
		return time.Time{}, false
	}
	suffix := strings.TrimPrefix(name, "nm-")
	if t, err := time.Parse("20060102T150405Z", suffix); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("20060102T150405Z0700", suffix); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

// WriteSource сохраняет маркер manual|schedule рядом с каталогом бэкапа.
func (d *DirStore) WriteSource(name, source string) error {
	if !d.DirReady() {
		return fmt.Errorf("backup dir not ready")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid backup name")
	}
	src := backup.NormalizeSource(source)
	if src == "" {
		return fmt.Errorf("invalid backup source")
	}
	return os.WriteFile(filepath.Join(d.Root, name+".source"), []byte(src+"\n"), 0o600)
}

func (d *DirStore) WriteAuthTarball(name, dataDir string) error {
	if !d.DirReady() {
		return fmt.Errorf("backup dir not ready")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid backup name")
	}
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return fmt.Errorf("data dir empty")
	}
	outPath := filepath.Join(d.Root, name+".auth.tgz")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	// os.Root: Open/Walk только внутри dataDir — без symlink TOCTOU за пределы корня (gosec G122).
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return err
	}
	defer root.Close()

	return fs.WalkDir(root.FS(), ".", func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		// Не следуем symlink'ам: иначе в tar могли бы попасть пути вне dataDir.
		if de.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel := filepath.ToSlash(path)
		info, err := de.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if de.IsDir() {
			hdr.Name += "/"
			return tw.WriteHeader(hdr)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		rf, err := root.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, rf)
		_ = rf.Close()
		return copyErr
	})
}

func (d *DirStore) Exists(name string) bool {
	if !d.DirReady() || !nameRe.MatchString(name) {
		return false
	}
	st, err := os.Stat(filepath.Join(d.Root, name))
	return err == nil && st.IsDir()
}

// Attached — имя бэкапа, чьи данные сейчас в живых таблицах (пусто = никто).
func (d *DirStore) Attached() (string, error) {
	if !d.DirReady() {
		return "", nil
	}
	b, err := os.ReadFile(filepath.Join(d.Root, attachedFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	name := strings.TrimSpace(string(b))
	if !nameRe.MatchString(name) {
		return "", nil
	}
	return name, nil
}

func (d *DirStore) SetAttached(name string) error {
	if !d.DirReady() {
		return fmt.Errorf("backup dir not ready")
	}
	path := filepath.Join(d.Root, attachedFile)
	name = strings.TrimSpace(name)
	if name == "" {
		_ = os.Remove(path)
		return nil
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid backup name")
	}
	return os.WriteFile(path, []byte(name+"\n"), 0o600)
}

// Delete удаляет каталог бэкапа и парный *.auth.tgz (маркер attached не трогает).
func (d *DirStore) Delete(name string) error {
	if !d.DirReady() {
		return fmt.Errorf("backup dir not ready")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid backup name")
	}
	if !d.Exists(name) {
		return fmt.Errorf("backup not found")
	}
	if err := os.RemoveAll(filepath.Join(d.Root, name)); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(d.Root, name+".auth.tgz"))
	_ = os.Remove(filepath.Join(d.Root, name+".source"))
	return nil
}

func (d *DirStore) Prune(keep int) error {
	if !d.DirReady() {
		return nil
	}
	if keep < 1 {
		keep = 1
	}
	attached, _ := d.Attached()
	list, err := d.List()
	if err != nil {
		return err
	}
	kept := 0
	for _, e := range list {
		if e.Name == attached {
			continue // подключённый бэкап не выкидываем политикой keep
		}
		kept++
		if kept <= keep {
			continue
		}
		_ = os.RemoveAll(filepath.Join(d.Root, e.Name))
		_ = os.Remove(filepath.Join(d.Root, e.Name+".auth.tgz"))
		_ = os.Remove(filepath.Join(d.Root, e.Name+".source"))
	}
	return nil
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}
