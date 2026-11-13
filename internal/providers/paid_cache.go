package providers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const paidCacheTTL = 24 * time.Hour

// SetCacheDir enables a best-effort private disk cache for subsequent requests.
// An empty directory or REM_NO_CACHE disables it.
func (p *Paid) SetCacheDir(dir string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cacheDir = dir
	if os.Getenv("REM_NO_CACHE") != "" {
		p.cacheDir = ""
	}
}

func (p *Paid) cachePath(requestURL string) string {
	p.mu.Lock()
	dir := p.cacheDir
	p.mu.Unlock()
	if dir == "" {
		return ""
	}
	id := sha256.Sum256([]byte("rem-paid-v1\x00" + p.key + "\x00" + requestURL))
	return filepath.Join(dir, hex.EncodeToString(id[:])+".json")
}

func (p *Paid) readCache(path string) paidObject {
	if path == "" {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > paidMaxBody {
		return nil
	}
	age := p.now().Sub(info.ModTime())
	if age < 0 || age >= paidCacheTTL {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, paidMaxBody+1))
	if err != nil || len(data) > paidMaxBody {
		return nil
	}
	body := paidDecode(data)
	if body == nil || paidRejected(body) || paidHasSecret(body, p.key) {
		return nil
	}
	return body
}

func (p *Paid) writeCache(path string, body paidObject) {
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(body)
	if err != nil || len(data) > paidMaxBody {
		return
	}
	file, err := os.CreateTemp(dir, ".rem-paid-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	if closeErr := file.Close(); writeErr != nil || closeErr != nil {
		return
	}
	now := p.now()
	if err := os.Chtimes(file.Name(), now, now); err != nil {
		return
	}
	_ = os.Rename(file.Name(), path)
}

func paidDecode(data []byte) paidObject {
	var body paidObject
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&body) != nil || body == nil || decoder.Decode(new(any)) != io.EOF {
		return nil
	}
	return body
}

func paidRejected(body paidObject) bool {
	success, ok := body["success"].(bool)
	return ok && !success
}

func paidHasSecret(raw any, key string) bool {
	if key == "" {
		return false
	}
	if object := paidObj(raw); object != nil {
		for name, value := range object {
			if strings.Contains(name, key) || paidHasSecret(value, key) {
				return true
			}
		}
	}
	switch value := raw.(type) {
	case string:
		return strings.Contains(value, key)
	case []any:
		for _, item := range value {
			if paidHasSecret(item, key) {
				return true
			}
		}
	}
	return false
}
