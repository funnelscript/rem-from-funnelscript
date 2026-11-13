package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

var runID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,119}$`)

const maxFile = 32 << 20

type Store struct{ root string }

func New(root string) *Store { return &Store{root: root} }

func (s *Store) CacheDir() string {
	return filepath.Join(filepath.Dir(s.root), "cache", "scrapecreators")
}

func (s *Store) Save(b model.Bundle, md string) (model.Bundle, error) {
	if b.ID == "" {
		var raw [12]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return model.Bundle{}, err
		}
		b.ID = "run-" + hex.EncodeToString(raw[:])
	}
	dir, err := s.path(b.ID)
	if err != nil {
		return model.Bundle{}, err
	}
	if err = os.MkdirAll(s.root, 0700); err != nil {
		return model.Bundle{}, err
	}
	tmp, err := os.MkdirTemp(s.root, ".pending-")
	if err != nil {
		return model.Bundle{}, err
	}
	defer os.RemoveAll(tmp)
	if err = Export(tmp, b, md); err != nil {
		return model.Bundle{}, err
	}
	if err = os.Rename(tmp, dir); err != nil {
		return model.Bundle{}, err
	}
	return b, nil
}

func Export(dir string, b model.Bundle, md string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	if len(data) >= maxFile || len(md) >= maxFile {
		return errors.New("output exceeds maximum size")
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err = writeAtomic(filepath.Join(dir, "report.md"), []byte(md)); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "evidence.json"), append(data, '\n'))
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (s *Store) path(id string) (string, error) {
	if !runID.MatchString(id) || strings.HasSuffix(id, ".") || strings.Contains(id, "..") {
		return "", errors.New("invalid run ID")
	}
	return filepath.Join(s.root, id), nil
}

func (s *Store) Load(id string) (model.Bundle, error) {
	path, err := s.path(id)
	if err != nil {
		return model.Bundle{}, err
	}
	return ReadBundle(filepath.Join(path, "evidence.json"))
}

func ReadBundle(path string) (model.Bundle, error) {
	var b model.Bundle
	raw, err := readBounded(path)
	if err != nil {
		return b, err
	}
	if err = json.Unmarshal(raw, &b); err != nil {
		return b, err
	}
	if b.Target.Input == "" && b.Target.Domain == "" && b.Brand == "" {
		return b, errors.New("file is not an evidence bundle")
	}
	return b, nil
}

func (s *Store) Report(id string) (string, error) {
	path, err := s.path(id)
	if err != nil {
		return "", err
	}
	body, err := readBounded(filepath.Join(path, "report.md"))
	return string(body), err
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() >= maxFile {
		return nil, errors.New("expected a bounded regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxFile))
	if len(b) >= maxFile {
		return nil, errors.New("file exceeds maximum size")
	}
	return b, err
}

func (s *Store) List() ([]model.Summary, error) {
	rows := []model.Summary{}
	files, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return rows, nil
	}
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if !f.IsDir() || !runID.MatchString(f.Name()) {
			continue
		}
		b, err := s.Load(f.Name())
		if err != nil {
			return nil, fmt.Errorf("read run %s: %w", f.Name(), err)
		}
		rows = append(rows, model.Summary{ID: f.Name(), Brand: b.Brand, Domain: b.Target.Domain, GeneratedAt: b.GeneratedAt, EvidenceCount: len(b.Evidence), UnknownCount: len(b.Unknowns)})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].GeneratedAt == rows[j].GeneratedAt {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].GeneratedAt > rows[j].GeneratedAt
	})
	return rows, nil
}

func (s *Store) LatestPair(domain string) (model.Bundle, model.Bundle, error) {
	rows, err := s.List()
	if err != nil {
		return model.Bundle{}, model.Bundle{}, err
	}
	var ids []string
	domain = strings.ToLower(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "www."))
	for _, row := range rows {
		if row.Domain == domain {
			ids = append(ids, row.ID)
			continue
		}
		b, e := s.Load(row.ID)
		if e != nil {
			return model.Bundle{}, model.Bundle{}, e
		}
		for _, alias := range b.Target.Aliases {
			if alias == domain {
				ids = append(ids, row.ID)
				break
			}
		}
	}
	if len(ids) < 2 {
		return model.Bundle{}, model.Bundle{}, fmt.Errorf("need two saved runs for %s (found %d)", domain, len(ids))
	}
	a, err := s.Load(ids[1])
	if err != nil {
		return a, model.Bundle{}, err
	}
	b, err := s.Load(ids[0])
	return a, b, err
}
