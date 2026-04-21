// Command debug-reports is a small read/maintenance CLI for the debug_reports
// table. It connects to the same database as the server (via DATABASE_URL or
// a local SQLite file) and supports three subcommands:
//
//	list   — list recent reports with optional --user / --since / --limit filters.
//	view   — pretty-print a single report with a color-coded breadcrumb feed.
//	purge  — delete reports older than a duration, with a y/N confirmation.
//
// Intentionally minimal: single file, stdlib only, no tablewriter / no color
// library.  TTY detection uses github.com/mattn/go-isatty which is already in
// the module graph as an indirect dependency.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/apsv/goal-tracker/backend/internal/db"
)

const usage = `debug-reports — inspect and maintain the debug_reports table.

Usage:
  debug-reports list  [--user EMAIL] [--since DUR] [--limit N]
  debug-reports view  <report-id>
  debug-reports purge  --older-than DUR [--yes]

Connection:
  DATABASE_URL       postgres://... — when set, connects to Postgres.
                     Otherwise falls back to the local SQLite file at the
                     default path used by the server.

Duration units:
  Accepts Ns, Nm, Nh, and Nd (days). Other suffixes are rejected.
`

func main() {
	err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		if len(args) == 0 {
			return errors.New("a subcommand is required")
		}
		return nil
	}

	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdList(rest, stdout, stderr)
	case "view":
		return cmdView(rest, stdout, stderr)
	case "purge":
		return cmdPurge(rest, stdin, stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q (try --help)", sub)
	}
}

// openDB picks Postgres when DATABASE_URL is set; otherwise it opens the
// default SQLite file the server uses.  Exits non-zero (via caller) if both
// fail.  Migrate() is NOT called — the CLI is read/maintenance only.
func openDB() (db.Database, error) {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return db.NewPostgres(url)
	}
	return db.NewSQLite(db.DefaultDBPath())
}

// --- list ---------------------------------------------------------------

func cmdList(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		userEmail = fs.String("user", "", "filter by user email (resolved to user_id)")
		since     = fs.String("since", "", "only reports newer than this duration (e.g. 7d, 24h, 30m)")
		limit     = fs.Int("limit", 50, "max number of reports to return")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: debug-reports list [--user EMAIL] [--since DUR] [--limit N]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	database, err := openDB()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	filter := db.DebugReportFilter{Limit: *limit}

	// Cache of user_id -> email for nicer output.  Seeded with --user filter.
	emails := map[string]string{}

	if *userEmail != "" {
		u, err := database.GetUserByEmail(*userEmail)
		if err != nil {
			return fmt.Errorf("lookup user by email: %w", err)
		}
		if u == nil {
			return fmt.Errorf("no user with email %q", *userEmail)
		}
		filter.UserID = &u.ID
		emails[u.ID] = u.Email
	}

	if *since != "" {
		d, err := parseDuration(*since)
		if err != nil {
			return fmt.Errorf("--since: %w", err)
		}
		t := time.Now().Add(-d)
		filter.Since = &t
	}

	reports, err := database.ListDebugReports(filter)
	if err != nil {
		return fmt.Errorf("list debug reports: %w", err)
	}

	fmt.Fprintf(stdout, "id\tcreated_at\tuser_email\ttrigger\tdescription_snippet\tapp_version\n")
	for _, r := range reports {
		email, ok := emails[r.UserID]
		if !ok {
			if u, err := database.GetUserByID(r.UserID); err == nil && u != nil {
				email = u.Email
			} else {
				email = "(unknown)"
			}
			emails[r.UserID] = email
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.ID,
			r.CreatedAt.UTC().Format(time.RFC3339),
			email,
			r.Trigger,
			descriptionSnippet(r.Description, 60),
			r.AppVersion,
		)
	}
	return nil
}

// --- view ---------------------------------------------------------------

// breadcrumb mirrors the frontend shape documented in the design doc.
type breadcrumb struct {
	TS       int64           `json:"ts"` // epoch millis
	Category string          `json:"category"`
	Level    string          `json:"level"`
	Message  string          `json:"message"`
	Data     json.RawMessage `json:"data,omitempty"`
}

const (
	ansiReset  = "\x1b[0m"
	ansiGray   = "\x1b[90m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
)

func cmdView(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("view", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: debug-reports view <report-id>")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("view requires exactly one report-id argument")
	}
	id := fs.Arg(0)

	database, err := openDB()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	r, err := database.GetDebugReport(id)
	if err != nil {
		return fmt.Errorf("get debug report: %w", err)
	}
	if r == nil {
		return fmt.Errorf("no debug report with id %q", id)
	}

	email := "(unknown)"
	if u, err := database.GetUserByID(r.UserID); err == nil && u != nil {
		email = u.Email
	}

	fmt.Fprintf(stdout, "Report:     %s\n", r.ID)
	fmt.Fprintf(stdout, "Created:    %s\n", r.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(stdout, "User:       %s (%s)\n", email, r.UserID)
	fmt.Fprintf(stdout, "Client ID:  %s\n", r.ClientID)
	fmt.Fprintf(stdout, "Trigger:    %s\n", r.Trigger)
	fmt.Fprintf(stdout, "Version:    %s\n", r.AppVersion)
	fmt.Fprintf(stdout, "Platform:   %s\n", r.Platform)
	fmt.Fprintf(stdout, "Device:     %s\n", compactJSON(r.Device))
	fmt.Fprintf(stdout, "State:      %s\n", compactJSON(r.State))
	if r.Description != "" {
		fmt.Fprintf(stdout, "\nDescription:\n%s\n", r.Description)
	}
	fmt.Fprintln(stdout, "\nBreadcrumbs:")

	var crumbs []breadcrumb
	if len(r.Breadcrumbs) > 0 {
		if err := json.Unmarshal(r.Breadcrumbs, &crumbs); err != nil {
			// Fall back to raw payload when the ring buffer is malformed
			// rather than blocking operator inspection.
			fmt.Fprintf(stdout, "  (failed to parse as []Breadcrumb: %v)\n  raw: %s\n", err, string(r.Breadcrumbs))
			return nil
		}
	}
	sort.SliceStable(crumbs, func(i, j int) bool { return crumbs[i].TS < crumbs[j].TS })

	color := useColor(stdout)
	for _, b := range crumbs {
		fmt.Fprintln(stdout, formatBreadcrumb(b, color))
		if hasData(b.Data) {
			fmt.Fprintf(stdout, "    %s\n", compactJSON(b.Data))
		}
	}
	return nil
}

// useColor reports whether ANSI color codes should be emitted on stdout.
// Honours the NO_COLOR convention and requires a real TTY.
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}

// formatBreadcrumb renders a single breadcrumb line.  Extracted so it's
// testable without a live TTY.
func formatBreadcrumb(b breadcrumb, color bool) string {
	ts := time.UnixMilli(b.TS).UTC().Format(time.RFC3339)
	line := fmt.Sprintf("%s  %-5s  %-7s  %s", ts, strings.ToUpper(b.Level), b.Category, b.Message)
	if !color {
		return line
	}
	switch b.Level {
	case "error":
		return ansiRed + line + ansiReset
	case "warn":
		return ansiYellow + line + ansiReset
	default:
		return ansiGray + line + ansiReset
	}
}

func hasData(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null" && s != "{}"
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "(empty)"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

// --- purge --------------------------------------------------------------

func cmdPurge(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("purge", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		olderThan = fs.String("older-than", "", "purge reports older than this duration (required; e.g. 90d)")
		yes       = fs.Bool("yes", false, "skip confirmation prompt")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: debug-reports purge --older-than DUR [--yes]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *olderThan == "" {
		fs.Usage()
		return errors.New("--older-than is required")
	}

	d, err := parseDuration(*olderThan)
	if err != nil {
		return fmt.Errorf("--older-than: %w", err)
	}
	cutoff := time.Now().Add(-d)

	if !*yes {
		fmt.Fprintf(stdout, "Delete all debug reports older than %s (cutoff: %s)? [y/N]: ", *olderThan, cutoff.UTC().Format(time.RFC3339))
		reader := bufio.NewReader(stdin)
		line, _ := reader.ReadString('\n')
		ans := strings.TrimSpace(line)
		if ans != "y" && ans != "Y" {
			fmt.Fprintln(stdout, "Aborted.")
			return nil
		}
	}

	database, err := openDB()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	n, err := database.DeleteOldDebugReports(cutoff)
	if err != nil {
		return fmt.Errorf("delete old debug reports: %w", err)
	}
	fmt.Fprintf(stdout, "Deleted %d reports.\n", n)
	return nil
}

// --- helpers -----------------------------------------------------------

// maxDurationDays caps --since/--older-than at 100 years.  Anything larger is
// almost certainly a typo, and unchecked int64 arithmetic below would wrap
// silently into a negative time.Duration — which for `purge` would flip the
// cutoff into the future and delete every row.  See `TestParseDuration`.
const maxDurationDays = 36500

// parseDuration extends time.ParseDuration with a `Nd` suffix (days).  Only
// a single unit is accepted: "7d", "24h", "30m", "45s".  Anything else
// (including mixed forms like "1h30m") is rejected.  This mirrors the CLI's
// narrow need — operators reach for it as a rough time window, not as a
// general duration DSL.
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration")
	}
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q: expected Nd/Nh/Nm/Ns", s)
	}
	unit := s[len(s)-1]
	num := s[:len(s)-1]

	// Reject leading sign — negative durations make no sense for --since/--older-than.
	if num == "" || num[0] == '-' || num[0] == '+' {
		return 0, fmt.Errorf("invalid duration %q: expected Nd/Nh/Nm/Ns", s)
	}

	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("invalid duration %q: must be non-negative", s)
	}

	switch unit {
	case 'd':
		if n > maxDurationDays {
			return 0, fmt.Errorf("duration out of range: %s (max %dd)", s, maxDurationDays)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		if n > maxDurationDays*24 {
			return 0, fmt.Errorf("duration out of range: %s (max %dh)", s, maxDurationDays*24)
		}
		return time.Duration(n) * time.Hour, nil
	case 'm':
		if n > maxDurationDays*24*60 {
			return 0, fmt.Errorf("duration out of range: %s (max %dm)", s, maxDurationDays*24*60)
		}
		return time.Duration(n) * time.Minute, nil
	case 's':
		if n > maxDurationDays*24*60*60 {
			return 0, fmt.Errorf("duration out of range: %s (max %ds)", s, maxDurationDays*24*60*60)
		}
		return time.Duration(n) * time.Second, nil
	default:
		return 0, fmt.Errorf("invalid duration unit %q: expected d/h/m/s", string(unit))
	}
}

// descriptionSnippet returns the first `max` runes of s with newlines
// collapsed to spaces and a trailing ellipsis when truncation occurred.
// Rune-safe so we don't split multibyte codepoints in the middle.
func descriptionSnippet(s string, max int) string {
	// Collapse any whitespace run (including \n and \r) to a single space.
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		b.WriteRune(r)
	}
	cleaned := strings.TrimSpace(b.String())

	runes := []rune(cleaned)
	if len(runes) <= max {
		return cleaned
	}
	return string(runes[:max]) + "\u2026"
}
