package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/al4danim/tick-tui/internal/config"
	"github.com/al4danim/tick-tui/internal/store"
)

const addUsage = `usage: tick add [--json] [--file PATH]

Batch-add pending tasks. Reads from stdin by default.

Default (plain text):
  One task per line in Obsidian-native format:
    买菜 @家庭
    write report @work
  Empty lines and lines starting with '#' are ignored.

--json:
  Stdin is a JSON array:
    [{"title":"buy milk","project":"home"},{"title":"write report"}]
  Fields: title (required), project (optional), date (optional, YYYY-MM-DD).

--file PATH:
  Read input from PATH instead of stdin.
`

func runAdd(args []string) int {
	var (
		asJSON  bool
		inFile  string
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json", "-j":
			asJSON = true
		case "--file", "-f":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "tick add: --file requires a path")
				return 2
			}
			inFile = args[i+1]
			i++
		case "-h", "--help":
			fmt.Print(addUsage)
			return 0
		default:
			fmt.Fprintf(os.Stderr, "tick add: unknown arg %q\n\n%s", args[i], addUsage)
			return 2
		}
	}

	var reader io.Reader = os.Stdin
	if inFile != "" {
		f, err := os.Open(inFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tick add: open %s: %v\n", inFile, err)
			return 1
		}
		defer f.Close()
		reader = f
	}

	items, err := parseAddInput(reader, asJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick add: %v\n", err)
		return 1
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stderr, "tick add: no tasks to add")
		return 0
	}

	cfgPath := config.DefaultPath()
	if !config.Exists(cfgPath) {
		fmt.Fprintln(os.Stderr, "tick add: no config found — run `tick` once to set up the tasks file")
		return 1
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick add: load config: %v\n", err)
		return 1
	}
	tasksFile := cfg.TasksFile
	if env := os.Getenv("TICK_TASKS_FILE"); env != "" {
		tasksFile = env
	}

	s, err := store.New(tasksFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick add: open store: %v\n", err)
		return 1
	}

	created, err := s.CreateBatch(items)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tick add: %v\n", err)
		return 1
	}

	fmt.Printf("added %d task(s) to %s\n", len(created), tasksFile)
	for _, f := range created {
		proj := ""
		if f.ProjectName != nil {
			proj = " @" + *f.ProjectName
		}
		fmt.Printf("  [%s] %s%s\n", f.ID, f.Title, proj)
	}
	return 0
}

func parseAddInput(r io.Reader, asJSON bool) ([]store.NewTask, error) {
	if asJSON {
		var raw []struct {
			Title   string `json:"title"`
			Project string `json:"project"`
			Date    string `json:"date"`
		}
		dec := json.NewDecoder(r)
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
		items := make([]store.NewTask, 0, len(raw))
		for i, x := range raw {
			t := strings.TrimSpace(x.Title)
			if t == "" {
				return nil, fmt.Errorf("item %d: empty title", i)
			}
			items = append(items, store.NewTask{Title: t, Project: strings.TrimSpace(x.Project), Date: strings.TrimSpace(x.Date)})
		}
		return items, nil
	}

	var items []store.NewTask
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		items = append(items, store.NewTask{Title: line})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return items, nil
}
