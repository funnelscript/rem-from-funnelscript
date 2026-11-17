package kb

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReportData represents the compiled knowledge base ready for HTML rendering.
type ReportData struct {
	Product       string         `json:"product"`
	GeneratedAt   string         `json:"generatedAt"`
	Charter       string         `json:"charter"`
	Status        string         `json:"status"`
	Screens       []ReportItem   `json:"screens"`
	Components    []ReportItem   `json:"components"`
	Rules         []ReportItem   `json:"rules"`
	Entities      []ReportItem   `json:"entities"`
	Barriers      []Barrier      `json:"barriers"`
	Tokens        string         `json:"tokens"`
	BuildBrief    string         `json:"buildBrief"`
	LedgerSummary map[string]int `json:"ledgerSummary"`
	Stats         map[string]int `json:"stats"`
}

// ReportItem holds title and body of an artifact.
type ReportItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	File    string `json:"file"`
	Content string `json:"content"`
}

const reportHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:;">
  <title>{{.Product}} · Reverse Engineering Knowledge Base Report</title>
  <style>
    :root {
      --bg: #090a0f;
      --card-bg: #12141c;
      --border: #232736;
      --text: #f0f2f8;
      --text-muted: #8b92a5;
      --primary: #3b82f6;
      --accent: #10b981;
      --warning: #f59e0b;
      --danger: #ef4444;
      --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      --code-font: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { background: var(--bg); color: var(--text); font-family: var(--font); line-height: 1.6; display: flex; height: 100vh; overflow: hidden; }
    #sidebar { width: 300px; background: var(--card-bg); border-right: 1px solid var(--border); display: flex; flex-direction: column; flex-shrink: 0; }
    .brand { padding: 20px; border-bottom: 1px solid var(--border); }
    .brand h1 { font-size: 1.15rem; font-weight: 700; color: #fff; letter-spacing: -0.02em; }
    .brand p { font-size: 0.8rem; color: var(--text-muted); margin-top: 4px; }
    .search-box { padding: 12px 16px; border-bottom: 1px solid var(--border); }
    .search-box input { width: 100%; background: #0c0d13; border: 1px solid var(--border); border-radius: 6px; padding: 8px 12px; color: var(--text); font-size: 0.85rem; outline: none; }
    .nav-links { list-style: none; overflow-y: auto; flex: 1; padding: 12px 0; }
    .nav-links li a { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; color: var(--text-muted); text-decoration: none; font-size: 0.9rem; font-weight: 500; transition: all 0.15s ease; }
    .nav-links li a:hover, .nav-links li.active a { color: #fff; background: rgba(59, 130, 246, 0.1); border-left: 3px solid var(--primary); }
    .badge { font-size: 0.75rem; padding: 2px 7px; border-radius: 10px; background: var(--border); color: var(--text); font-family: var(--code-font); }
    #main { flex: 1; overflow-y: auto; padding: 32px 48px; background: #0c0e14; }
    .tab-content { display: none; max-width: 960px; margin: 0 auto; }
    .tab-content.active { display: block; }
    h2 { font-size: 1.6rem; font-weight: 700; margin-bottom: 20px; color: #fff; border-bottom: 1px solid var(--border); padding-bottom: 12px; }
    h3 { font-size: 1.2rem; font-weight: 600; margin: 24px 0 12px 0; color: #d1d5db; }
    p { margin-bottom: 14px; font-size: 0.95rem; color: #e5e7eb; }
    pre { background: #06070a; border: 1px solid var(--border); border-radius: 8px; padding: 16px; overflow-x: auto; font-family: var(--code-font); font-size: 0.85rem; margin-bottom: 20px; color: #cbd5e1; }
    table { width: 100%; border-collapse: collapse; margin-bottom: 24px; font-size: 0.9rem; }
    th, td { border: 1px solid var(--border); padding: 10px 14px; text-align: left; }
    th { background: #161822; color: #9ca3af; font-weight: 600; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.05em; }
    tr:nth-child(even) { background: rgba(255,255,255,0.015); }
    .card { background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px; padding: 20px; margin-bottom: 20px; }
    .barrier-pill { display: inline-block; padding: 3px 8px; border-radius: 4px; font-size: 0.75rem; font-weight: 600; text-transform: uppercase; }
    .barrier-blocks-launch { background: rgba(239, 68, 68, 0.2); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.4); }
    .barrier-moat { background: rgba(16, 185, 129, 0.2); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.4); }
    .barrier-time { background: rgba(245, 158, 11, 0.2); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.4); }
  </style>
</head>
<body>
  <div id="sidebar">
    <div class="brand">
      <h1>{{.Product}}</h1>
      <p>Reverse-Engineering KB · Generated {{.GeneratedAt}}</p>
    </div>
    <div class="search-box">
      <input type="text" id="filterInput" placeholder="Filter sections & IDs...">
    </div>
    <ul class="nav-links">
      <li class="active"><a href="#charter" onclick="showTab('charter')"><span>0. Charter & Status</span></a></li>
      <li><a href="#screens" onclick="showTab('screens')"><span>1. Screens & UX</span><span class="badge">{{len .Screens}}</span></a></li>
      <li><a href="#components" onclick="showTab('components')"><span>2. Components</span><span class="badge">{{len .Components}}</span></a></li>
      <li><a href="#rules" onclick="showTab('rules')"><span>3. Business Rules</span><span class="badge">{{len .Rules}}</span></a></li>
      <li><a href="#datamodel" onclick="showTab('datamodel')"><span>4. Data Model</span><span class="badge">{{len .Entities}}</span></a></li>
      <li><a href="#barriers" onclick="showTab('barriers')"><span>5. Launch Barriers</span><span class="badge">{{len .Barriers}}</span></a></li>
      <li><a href="#tokens" onclick="showTab('tokens')"><span>6. Design Tokens</span></a></li>
      <li><a href="#build" onclick="showTab('build')"><span>7. Build Brief (BUILD.md)</span></a></li>
    </ul>
  </div>
  <div id="main">
    <div id="tab-charter" class="tab-content active">
      <h2>Project Charter & Status</h2>
      <div class="card">
        <h3>Charter</h3>
        <pre>{{.Charter}}</pre>
      </div>
      <div class="card">
        <h3>Live Status & Checkpoint</h3>
        <pre>{{.Status}}</pre>
      </div>
    </div>

    <div id="tab-screens" class="tab-content">
      <h2>Screens & State Machines</h2>
      {{range .Screens}}
        <div class="card item-card" data-search="{{.ID}} {{.Title}}">
          <h3>{{.ID}}: {{.Title}}</h3>
          <p><strong>Source:</strong> <code>{{.File}}</code></p>
          <pre>{{.Content}}</pre>
        </div>
      {{else}}
        <p>No screen artifacts recorded yet. Use <code>_templates/screen.md</code> to document identified screens.</p>
      {{end}}
    </div>

    <div id="tab-components" class="tab-content">
      <h2>Components & Micro-Interactions</h2>
      {{range .Components}}
        <div class="card item-card" data-search="{{.ID}} {{.Title}}">
          <h3>{{.ID}}: {{.Title}}</h3>
          <p><strong>Source:</strong> <code>{{.File}}</code></p>
          <pre>{{.Content}}</pre>
        </div>
      {{else}}
        <p>No component artifacts recorded yet.</p>
      {{end}}
    </div>

    <div id="tab-rules" class="tab-content">
      <h2>Given / When / Then Business Rules</h2>
      {{range .Rules}}
        <div class="card item-card" data-search="{{.ID}} {{.Title}}">
          <h3>{{.ID}}: {{.Title}}</h3>
          <p><strong>Source:</strong> <code>{{.File}}</code></p>
          <pre>{{.Content}}</pre>
        </div>
      {{else}}
        <p>No business rules recorded yet.</p>
      {{end}}
    </div>

    <div id="tab-datamodel" class="tab-content">
      <h2>Data Model & Entities</h2>
      {{range .Entities}}
        <div class="card item-card" data-search="{{.ID}} {{.Title}}">
          <h3>{{.ID}}: {{.Title}}</h3>
          <p><strong>Source:</strong> <code>{{.File}}</code></p>
          <pre>{{.Content}}</pre>
        </div>
      {{else}}
        <p>No entity models recorded yet.</p>
      {{end}}
    </div>

    <div id="tab-barriers" class="tab-content">
      <h2>Launch Barriers, Regulatory Gates & Moats</h2>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Title</th>
            <th>Category</th>
            <th>Severity</th>
            <th>Lead Time</th>
            <th>Mitigation</th>
          </tr>
        </thead>
        <tbody>
          {{range .Barriers}}
            <tr class="item-card" data-search="{{.ID}} {{.Title}} {{.Category}}">
              <td><code>{{.ID}}</code></td>
              <td><strong>{{.Title}}</strong></td>
              <td>{{.Category}}</td>
              <td>
                <span class="barrier-pill {{if eq .Severity "blocks launch"}}barrier-blocks-launch{{else if eq .Severity "moat"}}barrier-moat{{else}}barrier-time{{end}}">
                  {{.Severity}}
                </span>
              </td>
              <td>{{.LeadTime}}</td>
              <td>{{.Mitigation}}</td>
            </tr>
          {{else}}
            <tr><td colspan="6">No launch barriers cataloged in <code>clone/barriers.md</code>.</td></tr>
          {{end}}
        </tbody>
      </table>
    </div>

    <div id="tab-tokens" class="tab-content">
      <h2>Design Tokens & CSS Variables</h2>
      <pre>{{.Tokens}}</pre>
    </div>

    <div id="tab-build" class="tab-content">
      <h2>Coding Agent Handoff Brief (BUILD.md)</h2>
      <pre>{{.BuildBrief}}</pre>
    </div>
  </div>

  <script type="application/json" id="__rem_data">
    {{.SafeJSON}}
  </script>

  <script>
    function showTab(name) {
      document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
      document.querySelectorAll('.nav-links li').forEach(el => el.classList.remove('active'));
      const target = document.getElementById('tab-' + name);
      if (target) target.classList.add('active');
      const link = document.querySelector('a[href="#' + name + '"]');
      if (link && link.parentElement) link.parentElement.classList.add('active');
    }

    document.getElementById('filterInput')?.addEventListener('input', function(e) {
      const q = e.target.value.toLowerCase().trim();
      document.querySelectorAll('.item-card').forEach(el => {
        const text = (el.getAttribute('data-search') || el.textContent).toLowerCase();
        el.style.display = (!q || text.includes(q)) ? '' : 'none';
      });
    });

    // Check hash on load
    window.addEventListener('DOMContentLoaded', () => {
      const hash = window.location.hash.replace('#', '');
      if (hash) showTab(hash);
    });
  </script>
</body>
</html>`

// Report compiles the entire knowledge base into a self-contained HTML file.
func Report(kbDir string) (string, error) {
	absDir, err := filepath.Abs(kbDir)
	if err != nil {
		return "", err
	}

	charter, _ := os.ReadFile(filepath.Join(absDir, "00-charter.md"))
	status, _ := os.ReadFile(filepath.Join(absDir, "STATUS.md"))
	tokens, _ := os.ReadFile(filepath.Join(absDir, "design/tokens.md"))
	buildBrief, _ := os.ReadFile(filepath.Join(absDir, "BUILD.md"))

	product := filepath.Base(absDir)
	if len(charter) > 0 {
		for _, line := range strings.Split(string(charter), "\n") {
			if strings.HasPrefix(line, "# ") {
				product = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				product = strings.TrimPrefix(product, "Charter: ")
				break
			}
		}
	}

	data := ReportData{
		Product:       product,
		GeneratedAt:   time.Now().Format("2006-01-02 15:04:05 MST"),
		Charter:       string(charter),
		Status:        string(status),
		Tokens:        string(tokens),
		BuildBrief:    string(buildBrief),
		Screens:       loadItems(filepath.Join(absDir, "screens"), "S"),
		Components:    loadItems(filepath.Join(absDir, "components"), "C"),
		Rules:         loadItems(filepath.Join(absDir, "rules"), "R"),
		Entities:      loadItems(filepath.Join(absDir, "data-model/entities"), "ENT"),
		LedgerSummary: make(map[string]int),
		Stats:         make(map[string]int),
	}

	barriers, _ := ParseBarriers(absDir)
	data.Barriers = barriers

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"escapeHTML": html.EscapeString,
	}).Parse(reportHTMLTemplate)
	if err != nil {
		return "", fmt.Errorf("failed parsing report template: %w", err)
	}

	// Prepare SafeJSON escaping < and >
	rawJSON, _ := json.Marshal(data)
	safeJSONStr := strings.ReplaceAll(string(rawJSON), "<", "\\u003c")
	safeJSONStr = strings.ReplaceAll(safeJSONStr, ">", "\\u003e")

	type tmplWrapper struct {
		ReportData
		SafeJSON template.JS
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, tmplWrapper{
		ReportData: data,
		SafeJSON:   template.JS(safeJSONStr),
	}); err != nil {
		return "", fmt.Errorf("failed executing report template: %w", err)
	}

	return buf.String(), nil
}

func loadItems(dir, prefix string) []ReportItem {
	var items []ReportItem
	entries, err := os.ReadDir(dir)
	if err != nil {
		return items
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(raw)
		title := entry.Name()
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
				break
			}
		}
		items = append(items, ReportItem{
			ID:      strings.TrimSuffix(entry.Name(), ".md"),
			Title:   title,
			File:    entry.Name(),
			Content: content,
		})
	}
	return items
}
