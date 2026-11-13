package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/app"
	"github.com/funnelscript/rem-from-funnelscript/internal/mcp"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
	"github.com/funnelscript/rem-from-funnelscript/internal/report"
	"github.com/funnelscript/rem-from-funnelscript/internal/store"
	"github.com/funnelscript/rem-from-funnelscript/internal/web"
)

//go:embed demo/*
var demo embed.FS

const help = `REM From FunnelScript · public marketing research

Usage
  rem <domain-or-store-url> [--only website,seo] [--no-ai] [--json] [--out DIR]
  rem diff <domain> | rem diff <run-or-path-a> <run-or-path-b> [--json]
  rem validate <run-or-path> <report.md> [--json]
  rem web [--port 4321] [--no-open]
  rem demo [--port 4321] [--no-open]
  rem mcp
  rem doctor [--json]
  rem setup [--dry-run] [--yes] [--client claude_code,codex] [--json]

Investigation options
  --only       website,seo,history,apps,paid,social,founders,community
  --handle     repeatable platform=name override
  --max-credits per-run paid-provider cap, 0 to disable, default 50
  --engine     auto,evidence,anthropic,claude-code,codex
  --model      optional model for the selected writer
  --json       JSON to stdout, with the run also saved in history
  --out        export directory, default rem-reports/<run-id>
  -q, --quiet  suppress stage progress
  --home       override the history data directory
  -v, --version
`

type flags struct {
	help, version, noAI, json, quiet, noOpen, yes, dryRun bool
	out, only, engine, model, home, clients               string
	port, credits                                         int
	budgetSet                                             bool
	handles                                               map[string]string
}

func parse(args []string) (flags, []string, error) {
	f := flags{port: 4321, handles: map[string]string{}}
	if p := os.Getenv("REM_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			f.port = n
		}
	}
	var rest []string
	bools := map[string]*bool{"--help": &f.help, "-h": &f.help, "--version": &f.version, "-v": &f.version, "--no-ai": &f.noAI, "--json": &f.json, "--quiet": &f.quiet, "-q": &f.quiet, "--no-open": &f.noOpen, "--yes": &f.yes, "--dry-run": &f.dryRun}
	stringsByName := map[string]*string{"--out": &f.out, "--only": &f.only, "--engine": &f.engine, "--model": &f.model, "--home": &f.home, "--client": &f.clients}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			rest = append(rest, a)
			continue
		}
		name, value, inline := strings.Cut(a, "=")
		if p, ok := bools[name]; ok {
			if inline {
				return f, nil, fmt.Errorf("%s does not take a value", name)
			}
			*p = true
			continue
		}
		p, isString := stringsByName[name]
		if !isString && name != "--handle" && name != "--port" && name != "--max-credits" {
			return f, nil, fmt.Errorf("unknown option %q", name)
		}
		if !inline {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return f, nil, fmt.Errorf("%s requires a value", name)
			}
			i++
			value = args[i]
		}
		if isString {
			*p = value
			continue
		}
		if name == "--handle" {
			key, v, ok := strings.Cut(value, "=")
			if !ok || key == "" || v == "" {
				return f, nil, errors.New("handle must use platform=name")
			}
			f.handles[key] = v
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return f, nil, fmt.Errorf("%s requires an integer", name)
		}
		if name == "--port" {
			f.port = n
		} else {
			f.credits, f.budgetSet = n, true
		}
	}
	return f, rest, nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	f, rest, err := parse(args)
	if err != nil {
		return err
	}
	if f.version {
		fmt.Fprintln(out, model.Version)
		return nil
	}
	if f.help || len(rest) == 0 {
		fmt.Fprint(out, help)
		return nil
	}
	home := f.home
	if home == "" {
		home = os.Getenv("REM_HOME")
	}
	if home == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		home = filepath.Join(d, "reverse-engineer-marketing-go")
	}
	service := app.New(store.New(filepath.Join(home, "runs")))
	cmd, args := rest[0], rest[1:]
	switch cmd {
	case "mcp":
		if len(args) != 0 {
			return errors.New("usage: rem mcp")
		}
		return mcp.Serve(ctx, os.Stdin, out, service)
	case "web", "demo":
		if len(args) != 0 {
			return fmt.Errorf("usage: rem %s [--port PORT]", cmd)
		}
		route := ""
		if cmd == "demo" {
			const id = "demo-calai.app"
			if _, err := service.Store.Load(id); errors.Is(err, os.ErrNotExist) {
				var b model.Bundle
				raw, _ := demo.ReadFile("demo/evidence.json")
				if err = json.Unmarshal(raw, &b); err != nil {
					return err
				}
				b.ID = id
				md, _ := demo.ReadFile("demo/report.md")
				if _, err = service.Store.Save(b, string(md)); err != nil {
					if _, loadErr := service.Store.Load(id); loadErr != nil {
						return err
					}
				}
			} else if err != nil {
				return err
			}
			route = "/#/run/" + id
		}
		return serveWeb(ctx, service, f.port, f.noOpen, route, out)
	case "doctor":
		if len(args) != 0 {
			return errors.New("usage: rem doctor [--json]")
		}
		doctor := service.Doctor()
		if f.json {
			return printJSON(out, doctor)
		}
		fmt.Fprintf(out, "REM From FunnelScript %s\n\nPublic sources: %s\nScrape Creators key: %v\n", model.Version, strings.Join(doctor["freeSources"].([]string), ", "), doctor["scrapeCreators"])
		engines := doctor["engines"].(map[string]bool)
		for _, name := range []string{"evidence", "anthropic", "claude-code", "codex"} {
			fmt.Fprintf(out, "  %-12s %t\n", name, engines[name])
		}
		return nil
	case "setup":
		if len(args) != 0 {
			return errors.New("usage: rem setup [--dry-run] [--yes] [--client LIST]")
		}
		return setup(ctx, f, out)
	case "diff":
		return diff(service, args, f.json, out)
	case "validate":
		return validate(service, args, f.json, out)
	}
	if len(args) != 0 {
		return errors.New("pass one website or app store URL")
	}
	opts := app.Options{Handles: f.handles, MaxCredits: f.credits, BudgetSet: f.budgetSet, Engine: f.engine, Model: f.model}
	if !opts.BudgetSet {
		if value := os.Getenv("REM_MAX_CREDITS"); value != "" {
			opts.MaxCredits, err = strconv.Atoi(value)
			if err != nil {
				return errors.New("REM_MAX_CREDITS must be an integer")
			}
			opts.BudgetSet = true
		}
	}
	if f.only != "" {
		for _, a := range strings.Split(f.only, ",") {
			opts.Only = append(opts.Only, strings.TrimSpace(a))
		}
	}
	if f.noAI || f.json {
		opts.Engine = "evidence"
	}
	if !f.quiet && !f.json {
		opts.OnStage = func(stage, state string) { fmt.Fprintf(errOut, "%s · %s\n", stage, state) }
	}
	bundle, err := service.Investigate(ctx, cmd, opts)
	if err != nil {
		return err
	}
	if f.json {
		if err = printJSON(out, bundle); err != nil {
			return err
		}
	} else {
		md, err := service.Store.Report(bundle.ID)
		if err != nil {
			return err
		}
		dir := f.out
		if dir == "" {
			dir = filepath.Join("rem-reports", bundle.ID)
		}
		if err = store.Export(dir, bundle, md); err != nil {
			return fmt.Errorf("run %s saved in history, export failed: %w", bundle.ID, err)
		}
		path, err := filepath.Abs(filepath.Join(dir, "report.md"))
		if err != nil {
			return err
		}
		fmt.Fprintln(out, path)
		if bundle.ReportWarning != "" {
			fmt.Fprintln(errOut, "report writer:", bundle.ReportWarning)
		}
	}
	if len(bundle.Evidence) == 0 {
		return fmt.Errorf("no observations collected; inspect the recorded unknowns in run %s", bundle.ID)
	}
	return nil
}

func printJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}

func resolve(s *store.Store, ref string) (model.Bundle, error) {
	if strings.ContainsAny(ref, "/\\") || strings.HasSuffix(ref, ".json") {
		path := ref
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			path = filepath.Join(path, "evidence.json")
		}
		return store.ReadBundle(path)
	}
	return s.Load(ref)
}

func diff(s *app.Service, args []string, asJSON bool, out io.Writer) error {
	var a, b model.Bundle
	var err error
	switch len(args) {
	case 1:
		domain := args[0]
		if u, e := url.Parse(domain); e == nil && u.Hostname() != "" {
			domain = u.Hostname()
		}
		a, b, err = s.Store.LatestPair(domain)
	case 2:
		if a, err = resolve(s.Store, args[0]); err == nil {
			b, err = resolve(s.Store, args[1])
		}
	default:
		return errors.New("usage: rem diff <domain> | rem diff <run-or-path-a> <run-or-path-b>")
	}
	if err != nil {
		return err
	}
	if !sameTarget(a.Target, b.Target) {
		return errors.New("runs must refer to the same target")
	}
	d := report.Compare(a, b)
	if asJSON {
		return printJSON(out, d)
	}
	_, err = fmt.Fprint(out, report.RenderDiff(d))
	return err
}

func sameTarget(a, b model.Target) bool {
	if a.Domain != "" && b.Domain != "" {
		for _, x := range append([]string{a.Domain}, a.Aliases...) {
			for _, y := range append([]string{b.Domain}, b.Aliases...) {
				if strings.EqualFold(x, y) {
					return true
				}
			}
		}
	}
	return a.AppleID != "" && a.AppleID == b.AppleID || a.PlayID != "" && a.PlayID == b.PlayID
}

func validate(s *app.Service, args []string, asJSON bool, out io.Writer) error {
	if len(args) != 2 {
		return errors.New("usage: rem validate <run-or-path> <report.md>")
	}
	b, err := resolve(s.Store, args[0])
	if err != nil {
		return err
	}
	file, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer file.Close()
	const limit = 2 << 20
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return err
	}
	if len(raw) > limit {
		return errors.New("report exceeds size limit")
	}
	v := report.Validate(string(raw), b)
	if asJSON {
		if err = printJSON(out, v); err != nil {
			return err
		}
	} else {
		for _, c := range v.Checks {
			fmt.Fprintf(out, "%t · %s · %s\n", c.OK, c.Name, c.Detail)
		}
	}
	if !v.OK {
		return errors.New("report validation failed")
	}
	return nil
}

type setupStep struct {
	Client    string   `json:"client"`
	Available bool     `json:"available"`
	Args      []string `json:"args"`
}

func setup(ctx context.Context, f flags, out io.Writer) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	var plan []setupStep
	clients := f.clients
	if clients == "" {
		clients = "claude_code,codex"
	}
	for _, client := range strings.Split(clients, ",") {
		switch strings.TrimSpace(client) {
		case "claude_code":
			_, err := exec.LookPath("claude")
			plan = append(plan, setupStep{"claude", err == nil, []string{"mcp", "add", "--scope", "user", "reverse-engineer-marketing-go", "--", binary, "mcp"}})
		case "codex":
			_, err := exec.LookPath("codex")
			plan = append(plan, setupStep{"codex", err == nil, []string{"mcp", "add", "reverse-engineer-marketing-go", "--", binary, "mcp"}})
		default:
			return fmt.Errorf("unknown setup client %q", client)
		}
	}
	if f.json {
		if !f.dryRun && !f.yes {
			return errors.New("setup --json requires --dry-run or --yes")
		}
		if err = printJSON(out, plan); err != nil {
			return err
		}
	} else {
		for _, step := range plan {
			fmt.Fprintf(out, "%s · available=%t ·", step.Client, step.Available)
			for _, arg := range step.Args {
				fmt.Fprintf(out, " %q", arg)
			}
			fmt.Fprintln(out)
		}
	}
	if f.dryRun {
		return nil
	}
	if !f.yes {
		fmt.Fprint(out, "Apply to installed clients? [y/N] ")
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			return nil
		}
	}
	for _, step := range plan {
		if !step.Available {
			continue
		}
		cmd := exec.CommandContext(ctx, step.Client, step.Args...)
		if !f.json {
			cmd.Stdout = out
		}
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("%s registration failed: %w", step.Client, err)
		}
	}
	return nil
}

func serveWeb(ctx context.Context, s *app.Service, port int, noOpen bool, route string, out io.Writer) error {
	if port < 0 || port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}
	ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	address := "http://" + ln.Addr().String() + route
	srv := &http.Server{Handler: web.New(s), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 12 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 45 * time.Second, MaxHeaderBytes: 32 << 10}
	fmt.Fprintln(out, address)
	if !noOpen {
		switch runtime.GOOS {
		case "darwin":
			startBrowser("open", address)
		case "windows":
			startBrowser("rundll32", "url.dll,FileProtocolHandler", address)
		default:
			startBrowser("xdg-open", address)
		}
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
		}
		stop, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		_ = srv.Shutdown(stop)
	}()
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func startBrowser(bin string, args ...string) {
	cmd := exec.Command(bin, args...)
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}
