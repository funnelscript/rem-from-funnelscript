package kb

import (
	"bytes"
	"embed"
	"encoding/csv"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

//go:embed assets/templates/* assets/scripts/*
var assetsFS embed.FS

var (
	// Standard ID patterns: FL, SRC, EV, BR, F, S, C, R, ENT, B, Q, L
	// Also supports legacy E-### as an alias for EV-### in evidence checks.
	idPattern  = regexp.MustCompile(`\b(FL|SRC|EV|BR|F|S|C|R|ENT|E|B|Q|L)-(\d{3})\b`)
	tagPattern = regexp.MustCompile(`\[(obs|net|doc|inf)(?:\s+([A-Z]+-\d{3}))?\]`)
)

var validLedgerClasses = map[string]bool{
	"A1": true, "A2": true, "A3": true, "A4": true, "incident": true,
}

var validLedgerActions = map[string]bool{
	"create": true, "update": true, "delete": true, "toggle": true, "incident": true,
}

var validLedgerStatuses = map[string]bool{
	"open": true, "restored": true, "removed": true, "fixture": true, "kept": true, "human": true,
}

// CheckResult contains validation findings for a knowledge base.
type CheckResult struct {
	KBDir       string            `json:"kbDir"`
	Valid       bool              `json:"valid"`
	Closing     bool              `json:"closing"`
	Errors      []string          `json:"errors"`
	Warnings    []string          `json:"warnings"`
	Definitions map[string]string `json:"definitions"`
	Stats       map[string]int    `json:"stats"`
}

// Barrier represents a parsed launch barrier entry.
type Barrier struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	Severity     string `json:"severity"`
	MarketWeight string `json:"marketWeight"`
	LeadTime     string `json:"leadTime"`
	Cost         string `json:"cost"`
	Mitigation   string `json:"mitigation"`
	Evidence     string `json:"evidence"`
}

// Script returns the embedded browser script by name.
func Script(name string) (string, error) {
	cleanName := strings.TrimSuffix(name, ".js")
	path := fmt.Sprintf("assets/scripts/%s.js", cleanName)
	data, err := assetsFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("unknown script %q (available: inventory, tokens, mask, annotate)", cleanName)
	}
	return string(data), nil
}

// Slugify generates a clean filesystem slug from a product name.
func Slugify(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
		}
	}
	res := strings.Trim(b.String(), "-")
	if res == "" {
		return "product"
	}
	return res
}

// Init creates a new SaaS reverse-engineering knowledge base from embedded templates.
func Init(targetDir, product string, initGit bool) (string, error) {
	if product == "" {
		return "", errors.New("product name is required")
	}
	slug := Slugify(product)
	if targetDir == "" {
		targetDir = "re-" + slug
	}

	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	replacements := map[string]string{
		"PRODUCT": product,
		"SLUG":    slug,
		"DATE":    time.Now().Format("2006-01-02"),
	}

	// Copy kb templates
	if err := copyEmbedSubtree("assets/templates/kb", absDir, replacements); err != nil {
		return "", fmt.Errorf("failed copying kb templates: %w", err)
	}

	// Copy artifact templates to _templates/
	templatesDir := filepath.Join(absDir, "_templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		return "", err
	}
	if err := copyEmbedSubtree("assets/templates/artifacts", templatesDir, nil); err != nil {
		return "", fmt.Errorf("failed copying artifact templates: %w", err)
	}

	// Copy handoff BUILD.md if not present
	buildMdPath := filepath.Join(absDir, "BUILD.md")
	if _, err := os.Stat(buildMdPath); os.IsNotExist(err) {
		if rawBuild, err := assetsFS.ReadFile("assets/templates/handoff/BUILD.md"); err == nil {
			content := string(rawBuild)
			for k, v := range replacements {
				content = strings.ReplaceAll(content, "{{"+k+"}}", v)
			}
			_ = os.WriteFile(buildMdPath, []byte(content), 0o644)
		}
	}

	if initGit {
		_ = exec.Command("git", "-C", absDir, "init").Run()
	}

	return absDir, nil
}

func copyEmbedSubtree(srcDir, destDir string, replacements map[string]string) error {
	return fs.WalkDir(assetsFS, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		targetPath := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		// Don't overwrite existing files
		if _, statErr := os.Stat(targetPath); statErr == nil {
			return nil
		}

		data, err := assetsFS.ReadFile(path)
		if err != nil {
			return err
		}

		if replacements != nil && isTextFile(path) {
			content := string(data)
			for k, v := range replacements {
				content = strings.ReplaceAll(content, "{{"+k+"}}", v)
			}
			data = []byte(content)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	})
}

func isTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".csv", ".json", ".mmd", ".ts", ".js", ".txt", ".html":
		return true
	default:
		return false
	}
}

// Check validates an existing knowledge base directory.
func Check(kbDir string, closing bool) (*CheckResult, error) {
	absDir, err := filepath.Abs(kbDir)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("knowledge base directory %q does not exist", kbDir)
	}

	result := &CheckResult{
		KBDir:       absDir,
		Valid:       true,
		Closing:     closing,
		Errors:      []string{},
		Warnings:    []string{},
		Definitions: make(map[string]string),
		Stats:       make(map[string]int),
	}

	// 1. Required core files check
	requiredFiles := []string{
		"00-charter.md",
		"STATUS.md",
		"sandbox/environment.md",
		"sandbox/ledger.csv",
	}
	for _, req := range requiredFiles {
		p := filepath.Join(absDir, req)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("Missing required file: %s", req))
		}
	}

	// 2. Scan and index all files
	files := make([]string, 0)
	_ = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // ignore unreadable entries and continue walking
		}
		rel, _ := filepath.Rel(absDir, path)
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "_templates" {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, rel)
		return nil
	})

	// 3. Scan definitions
	for _, rel := range files {
		ext := strings.ToLower(filepath.Ext(rel))
		base := filepath.Base(rel)
		stem := strings.TrimSuffix(base, ext)

		// File name based IDs (e.g. S-001-dashboard.md, C-002-button.md, ENT-003-deal.md)
		if m := idPattern.FindStringSubmatch(stem); len(m) == 3 {
			id := fmt.Sprintf("%s-%s", m[1], m[2])
			if existing, found := result.Definitions[id]; found {
				result.Errors = append(result.Errors, fmt.Sprintf("Duplicate ID definition: %s in %s (already defined in %s)", id, rel, existing))
			} else {
				result.Definitions[id] = rel
				prefix := m[1]
				result.Stats[prefix]++
			}
		}

		if ext == ".md" {
			scanMarkdownDefinitions(absDir, rel, result)
		}
	}

	// 4. Validate sandbox/ledger.csv
	validateLedger(absDir, closing, result)

	// 5. Scan markdown files for unresolved references and evidence rules
	for _, rel := range files {
		if strings.ToLower(filepath.Ext(rel)) == ".md" {
			validateMarkdownContent(absDir, rel, result)
		}
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result, nil
}

func scanMarkdownDefinitions(absDir, rel string, result *CheckResult) {
	data, err := os.ReadFile(filepath.Join(absDir, rel))
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Match table rows starting with | ID |
		if strings.HasPrefix(trimmed, "|") {
			parts := strings.Split(trimmed, "|")
			if len(parts) >= 2 {
				firstCell := strings.TrimSpace(parts[1])
				if m := idPattern.FindStringSubmatch(firstCell); len(m) == 3 {
					id := fmt.Sprintf("%s-%s", m[1], m[2])
					if existing, found := result.Definitions[id]; found && existing != rel {
						result.Errors = append(result.Errors, fmt.Sprintf("Duplicate ID definition in table: %s in %s (already in %s)", id, rel, existing))
					} else {
						result.Definitions[id] = rel
						result.Stats[m[1]]++
					}
				}
			}
		}

		// Match headings: ## BR-001: Title or ### B-001: Finding
		if strings.HasPrefix(trimmed, "#") {
			if m := idPattern.FindStringSubmatch(trimmed); len(m) == 3 {
				id := fmt.Sprintf("%s-%s", m[1], m[2])
				if existing, found := result.Definitions[id]; found && existing != rel {
					result.Errors = append(result.Errors, fmt.Sprintf("Duplicate ID heading: %s in %s (already in %s)", id, rel, existing))
				} else {
					result.Definitions[id] = rel
					result.Stats[m[1]]++
				}
			}
		}
	}
}

func validateLedger(absDir string, closing bool, result *CheckResult) {
	ledgerPath := filepath.Join(absDir, "sandbox/ledger.csv")
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		return // already logged in required files
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("sandbox/ledger.csv is invalid CSV: %v", err))
		return
	}
	if len(rows) < 1 {
		result.Errors = append(result.Errors, "sandbox/ledger.csv is empty")
		return
	}

	header := rows[0]
	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[strings.TrimSpace(strings.ToLower(col))] = i
	}

	for _, required := range []string{"id", "class", "action", "status"} {
		if _, ok := colIdx[required]; !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("sandbox/ledger.csv missing required column %q", required))
		}
	}

	openRows := 0
	for lineNum, row := range rows[1:] {
		if len(row) == 0 || (len(row) == 1 && row[0] == "") {
			continue
		}
		id := getCol(row, colIdx, "id")
		class := getCol(row, colIdx, "class")
		action := getCol(row, colIdx, "action")
		status := getCol(row, colIdx, "status")

		if id != "" {
			if existing, found := result.Definitions[id]; found && existing != "sandbox/ledger.csv" {
				result.Errors = append(result.Errors, fmt.Sprintf("Duplicate ledger ID %s in sandbox/ledger.csv:line %d", id, lineNum+2))
			} else {
				result.Definitions[id] = "sandbox/ledger.csv"
				result.Stats["L"]++
			}
		}

		if class != "" && !validLedgerClasses[class] {
			result.Errors = append(result.Errors, fmt.Sprintf("sandbox/ledger.csv:line %d: invalid class %q", lineNum+2, class))
		}
		if action != "" && !validLedgerActions[action] {
			result.Errors = append(result.Errors, fmt.Sprintf("sandbox/ledger.csv:line %d: invalid action %q", lineNum+2, action))
		}
		if status != "" {
			if !validLedgerStatuses[status] {
				result.Errors = append(result.Errors, fmt.Sprintf("sandbox/ledger.csv:line %d: invalid status %q", lineNum+2, status))
			}
			if status == "open" {
				openRows++
			}
			if closing && (status == "open" || status == "fixture" || status == "human") {
				result.Errors = append(result.Errors, fmt.Sprintf("Closing check failed: unresolved ledger row %s has status %q (must be restored or removed)", id, status))
			}
		}
	}
	result.Stats["openLedgerRows"] = openRows
}

func getCol(row []string, colIdx map[string]int, name string) string {
	idx, ok := colIdx[name]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func validateMarkdownContent(absDir, rel string, result *CheckResult) {
	data, err := os.ReadFile(filepath.Join(absDir, rel))
	if err != nil {
		return
	}
	content := string(data)

	// Clean out code blocks and comments
	cleaned := stripCodeBlocks(content)

	// In rules/, ensure each rule file has an evidence tag
	if strings.HasPrefix(rel, "rules/") && strings.HasSuffix(rel, ".md") && !strings.Contains(rel, "_templates") {
		if !tagPattern.MatchString(cleaned) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: business rule lacks evidence tag [obs], [net], [doc], or [inf]", rel))
		}
	}

	// In clone/barriers.md, check barrier headings
	if rel == "clone/barriers.md" {
		lines := strings.Split(cleaned, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "### BR-") && !tagPattern.MatchString(line) {
				// Barrier headings may include citations on body lines; no warning required
				continue
			}
		}
	}
}

func stripCodeBlocks(text string) string {
	// Strip ``` ... ```
	inCode := false
	var out []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if !inCode && !strings.HasPrefix(trimmed, "<!--") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// ParseBarriers reads and structures barriers from clone/barriers.md.
func ParseBarriers(kbDir string) ([]Barrier, error) {
	path := filepath.Join(kbDir, "clone/barriers.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var barriers []Barrier
	lines := strings.Split(string(data), "\n")
	var current *Barrier

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### BR-") {
			if current != nil {
				barriers = append(barriers, *current)
			}
			parts := strings.SplitN(trimmed, ":", 2)
			idPart := strings.TrimPrefix(parts[0], "### ")
			title := ""
			if len(parts) > 1 {
				title = strings.TrimSpace(parts[1])
			}
			current = &Barrier{
				ID:    strings.TrimSpace(idPart),
				Title: title,
			}
			continue
		}
		if current == nil {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "- category:") {
			current.Category = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Category:"))
		} else if strings.HasPrefix(lower, "- severity:") {
			current.Severity = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Severity:"))
		} else if strings.HasPrefix(lower, "- market weight:") {
			current.MarketWeight = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Market weight:"))
		} else if strings.HasPrefix(lower, "- lead time:") {
			current.LeadTime = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Lead time:"))
		} else if strings.HasPrefix(lower, "- cost:") {
			current.Cost = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Cost:"))
		} else if strings.HasPrefix(lower, "- mitigation:") {
			current.Mitigation = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Mitigation:"))
		} else if strings.HasPrefix(lower, "- evidence:") {
			current.Evidence = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Evidence:"))
		}
	}
	if current != nil {
		barriers = append(barriers, *current)
	}

	return barriers, nil
}
