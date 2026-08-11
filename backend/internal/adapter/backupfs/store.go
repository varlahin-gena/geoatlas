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

var nameRe = regexp.MustCompile(`^nm-\d{8}T\d{6}Z$`)

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
		out = append(out, backup.Entry{
			Name:      name,
			CreatedAt: created,
			SizeBytes: size,
			HasAuth:   auth,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func parseBackupNameTime(name string) (time.Time, bool) {
	// nm-20060102T150405Z
	if !strings.HasPrefix(name, "nm-") {
		return time.Time{}, false
	}
	t, err := time.Parse("20060102T150405Z", strings.TrimPrefix(name, "nm-"))
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
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

	return filepath.WalkDir(dataDir, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
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
		rf, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, rf)
		_ = rf.Close()
		return copyErr
	})
}

func (d *DirStore) Prune(keep int) error {
	if !d.DirReady() {
		return nil
	}
	if keep < 1 {
		keep = 1
	}
	list, err := d.List()
	if err != nil {
		return err
	}
	for i, e := range list {
		if i < keep {
			continue
		}
		_ = os.RemoveAll(filepath.Join(d.Root, e.Name))
		_ = os.Remove(filepath.Join(d.Root, e.Name+".auth.tgz"))
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
