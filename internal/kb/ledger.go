package kb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	evidenceIDRegex  = regexp.MustCompile(`^(?:EV|E)-[0-9]+$`)
	evidenceRowRegex = regexp.MustCompile(`(?m)^\s*\|\s*((?:EV|E)-[0-9]+)\s*\|`)
)

var validTaskStatuses = map[string]bool{
	"pending":     true,
	"ready":       true,
	"in_progress": true,
	"blocked":     true,
	"done":        true,
}

// TaskRecord models a single task in TASKS.json.
type TaskRecord struct {
	ID           string   `json:"id"`
	Owner        string   `json:"owner"`
	Outcome      string   `json:"outcome"`
	Status       string   `json:"status"`
	DependsOn    []string `json:"depends_on"`
	EvidenceIDs  []string `json:"evidence_ids"`
	Checks       []string `json:"checks"`
	WritePaths   []string `json:"write_paths"`
	Verification string   `json:"verification,omitempty"`
}

// LedgerResult contains validation findings for a .re-ledger directory.
type LedgerResult struct {
	LedgerDir string   `json:"ledgerDir"`
	Valid     bool     `json:"valid"`
	TaskCount int      `json:"taskCount"`
	Errors    []string `json:"errors"`
	Warnings  []string `json:"warnings"`
}

// CheckLedger validates the .re-ledger directory structure, task DAG dependencies, cycles, and write path overlaps.
func CheckLedger(baseDir string) (*LedgerResult, error) {
	ledgerDir := baseDir
	// If baseDir contains a .re-ledger subfolder, use it
	if sub, err := os.Stat(filepath.Join(baseDir, ".re-ledger")); err == nil && sub.IsDir() {
		ledgerDir = filepath.Join(baseDir, ".re-ledger")
	} else if fi, err := os.Stat(ledgerDir); err != nil {
		return nil, fmt.Errorf("ledger directory not found: %s", baseDir)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("ledger path is not a directory: %s", baseDir)
	}

	result := &LedgerResult{
		LedgerDir: ledgerDir,
		Valid:     true,
		Errors:    []string{},
		Warnings:  []string{},
	}

	requiredFiles := []string{"STATUS.md", "EVIDENCE.md", "DECISIONS.md", "PARITY.md", "TASKS.json"}
	for _, f := range requiredFiles {
		p := filepath.Join(ledgerDir, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("Missing required file: %s", f))
		}
	}

	if len(result.Errors) > 0 {
		result.Valid = false
		return result, nil
	}

	// Read EVIDENCE.md to index valid evidence IDs
	evidencePath := filepath.Join(ledgerDir, "EVIDENCE.md")
	evidenceData, err := os.ReadFile(evidencePath)
	if err != nil {
		return nil, err
	}

	evidenceIDs := make(map[string]bool)
	matches := evidenceRowRegex.FindAllStringSubmatch(string(evidenceData), -1)
	for _, m := range matches {
		if len(m) > 1 {
			id := m[1]
			if evidenceIDs[id] {
				result.Errors = append(result.Errors, fmt.Sprintf("Duplicate evidence ID in EVIDENCE.md: %s", id))
			}
			evidenceIDs[id] = true
		}
	}

	// Read and parse TASKS.json (supports both { "version": 1, "tasks": [...] } and raw array [...])
	tasksPath := filepath.Join(ledgerDir, "TASKS.json")
	tasksData, err := os.ReadFile(tasksPath)
	if err != nil {
		return nil, err
	}

	var tasks []TaskRecord
	var envelope struct {
		Version int          `json:"version"`
		Tasks   []TaskRecord `json:"tasks"`
	}
	if err := json.Unmarshal(tasksData, &envelope); err == nil && envelope.Version > 0 {
		tasks = envelope.Tasks
	} else if parseErr := json.Unmarshal(tasksData, &tasks); parseErr != nil {
		result.Errors = append(result.Errors, "TASKS.json must have version 1 and a tasks array (or array of tasks)")
		result.Valid = false
		return result, nil //nolint:nilerr // structural parse error captured in result.Errors
	}

	result.TaskCount = len(tasks)

	graph := make(map[string][]string)
	statuses := make(map[string]string)
	type activePath struct {
		taskID string
		parts  []string
	}
	var activePaths []activePath

	for idx, t := range tasks {
		taskNum := idx + 1
		id := strings.TrimSpace(t.ID)
		if id == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("Task %d needs a non-empty id", taskNum))
			continue
		}
		if _, exists := graph[id]; exists {
			result.Errors = append(result.Errors, fmt.Sprintf("Duplicate task id: %s", id))
			continue
		}

		if strings.TrimSpace(t.Owner) == "" || strings.TrimSpace(t.Outcome) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: owner and outcome are required", id))
		}
		if !validTaskStatuses[t.Status] {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid status %q", id, t.Status))
		}
		if len(t.Checks) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: at least one verification command is required in checks", id))
		}

		for _, ref := range t.EvidenceIDs {
			if !evidenceIDRegex.MatchString(ref) || !evidenceIDs[ref] {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: unknown or invalid evidence ID %s", id, ref))
			}
		}

		for _, p := range t.WritePaths {
			parts := cleanPathParts(p)
			if len(parts) == 0 {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: write_paths must contain valid project-relative paths (got %q)", id, p))
			} else if t.Status == "ready" || t.Status == "in_progress" {
				activePaths = append(activePaths, activePath{taskID: id, parts: parts})
			}
		}

		if t.Status == "done" && strings.TrimSpace(t.Verification) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: completed tasks require a verification reference", id))
		}

		graph[id] = t.DependsOn
		statuses[id] = t.Status
	}

	// Validate dependency existence & status ordering
	for id, deps := range graph {
		for _, dep := range deps {
			if _, exists := graph[dep]; !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: depends on unknown task %s", id, dep))
			} else if (statuses[id] == "ready" || statuses[id] == "in_progress" || statuses[id] == "done") && statuses[dep] != "done" {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: task is %s but dependency %s is %s (must be done)", id, statuses[id], dep, statuses[dep]))
			}
		}
	}

	// Cycle detection using Kahn's algorithm
	if cycleErr := detectCycles(graph); cycleErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Cyclic dependency detected: %v", cycleErr))
	}

	// Write path conflict detection among concurrent active tasks
	for i := 0; i < len(activePaths); i++ {
		for j := i + 1; j < len(activePaths); j++ {
			if activePaths[i].taskID == activePaths[j].taskID {
				continue
			}
			if pathsOverlap(activePaths[i].parts, activePaths[j].parts) {
				result.Errors = append(result.Errors, fmt.Sprintf("Conflicting write_paths between concurrent active tasks %s and %s: %s vs %s",
					activePaths[i].taskID, activePaths[j].taskID,
					strings.Join(activePaths[i].parts, "/"), strings.Join(activePaths[j].parts, "/")))
			}
		}
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result, nil
}

func cleanPathParts(p string) []string {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "-") || strings.Contains(clean, ":") {
		return nil
	}
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part == ".." || part == "." || part == "" {
			return nil
		}
	}
	return parts
}

func pathsOverlap(a, b []string) bool {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func detectCycles(graph map[string][]string) error {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for id := range graph {
		inDegree[id] = 0
	}

	for id, deps := range graph {
		for _, dep := range deps {
			if _, exists := graph[dep]; exists {
				inDegree[id]++
				dependents[dep] = append(dependents[dep], id)
			}
		}
	}

	queue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	visitedCount := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visitedCount++

		for _, depTask := range dependents[curr] {
			inDegree[depTask]--
			if inDegree[depTask] == 0 {
				queue = append(queue, depTask)
			}
		}
	}

	if visitedCount < len(graph) {
		return errors.New("cycle exists in task dependency graph")
	}
	return nil
}
