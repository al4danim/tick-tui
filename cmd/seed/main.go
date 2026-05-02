// seed is a development tool that populates a tasks.md + archive.md pair
// with realistic fake data for testing the stats panel.
//
// Usage:
//
//	go run ./cmd/seed --days 365 --avg 5 --out /tmp/tick-demo
//	TICK_TASKS_FILE=/tmp/tick-demo/tasks.md ./bin/tick
package main

import (
	crand "crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var projects = []string{"work", "home", "reading", "ops"}

// pendingCount is how many synthetic pending tasks to add to tasks.md so the
// main TUI list isn't empty when previewing seeded data.
const pendingCount = 5

func main() {
	days := flag.Int("days", 365, "how many days of history to generate")
	avg := flag.Int("avg", 5, "average done tasks per day")
	out := flag.String("out", "", "output directory (required)")
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "seed: --out is required")
		os.Exit(1)
	}
	if err := run(*out, *days, *avg); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run(outDir string, days, avgPerDay int) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tasksPath := filepath.Join(outDir, "tasks.md")
	archivePath := filepath.Join(outDir, "archive.md")

	today := time.Now()
	cutoffDay := today.AddDate(0, 0, -7) // 7-day boundary: tasks.md vs archive.md

	var taskLines []string
	var archiveLines []string

	totalDone := 0
	maxCount := 0
	oldestDate := ""

	for i := 0; i < days; i++ {
		// i=0 is the oldest day; i=days-1 is today.
		d := today.AddDate(0, 0, -(days - 1 - i))
		dateStr := d.Format("2006-01-02")
		count := sampleCount(avgPerDay)
		if count > maxCount {
			maxCount = count
		}
		if count > 0 && oldestDate == "" {
			oldestDate = dateStr
		}
		totalDone += count

		for j := 0; j < count; j++ {
			line := makeDoneLine(dateStr)
			// Recent 7 days → tasks.md; older → archive.md
			if !d.Before(cutoffDay) {
				taskLines = append(taskLines, line)
			} else {
				archiveLines = append(archiveLines, line)
			}
		}
	}

	// Add pending tasks (recent creations, no *date) so the main list isn't empty.
	for i := 0; i < pendingCount; i++ {
		created := today.AddDate(0, 0, -rand.Intn(7))
		taskLines = append(taskLines, makePendingLine(created.Format("2006-01-02")))
	}

	if err := writeLines(tasksPath, taskLines); err != nil {
		return fmt.Errorf("write tasks.md: %w", err)
	}
	if err := writeLines(archivePath, archiveLines); err != nil {
		return fmt.Errorf("write archive.md: %w", err)
	}

	// Count how many done lines are in tasks.md (last 7 days).
	doneInTasks := len(taskLines) - pendingCount
	if doneInTasks < 0 {
		doneInTasks = 0
	}

	fmt.Printf("seeded %d done · %d pending across %d days (avg %.1f/day, max %d)\n",
		totalDone, pendingCount, days, float64(totalDone)/float64(days), maxCount)
	fmt.Printf("  → %s (%d done · %d pending in last 7 days)\n", tasksPath, doneInTasks, pendingCount)
	oldest := oldestDate
	if oldest == "" {
		oldest = "(none)"
	}
	fmt.Printf("  → %s (%d done · oldest %s)\n", archivePath, len(archiveLines), oldest)

	return nil
}

// sampleCount returns a random non-negative integer drawn from a truncated
// normal distribution centred on avg with stddev = avg/2.
// This produces realistic "spiky" distributions for the heatmap.
func sampleCount(avg int) int {
	if avg <= 0 {
		return 0
	}
	stddev := float64(avg) / 2.0
	// Box-Muller transform using two uniform randoms.
	u1 := rand.Float64()
	u2 := rand.Float64()
	if u1 == 0 {
		u1 = 1e-10
	}
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	v := float64(avg) + stddev*z
	// Round and clamp to [0, avg*3].
	n := int(math.Round(v))
	if n < 0 {
		return 0
	}
	if n > avg*3 {
		return avg * 3
	}
	return n
}

func makeDoneLine(date string) string {
	title := randomTitle()
	proj := ""
	if rand.Float64() < 0.8 {
		proj = projects[rand.Intn(len(projects))]
	}
	id := genID()
	var b strings.Builder
	b.WriteString("- [x] ")
	b.WriteString(title)
	if proj != "" {
		b.WriteString(" @")
		b.WriteString(proj)
	}
	b.WriteString(" +")
	b.WriteString(date)
	b.WriteString(" *")
	b.WriteString(date)
	b.WriteString(" [")
	b.WriteString(id)
	b.WriteString("]")
	return b.String()
}

func makePendingLine(created string) string {
	title := randomTitle()
	proj := ""
	if rand.Float64() < 0.8 {
		proj = projects[rand.Intn(len(projects))]
	}
	id := genID()
	var b strings.Builder
	b.WriteString("- [ ] ")
	b.WriteString(title)
	if proj != "" {
		b.WriteString(" @")
		b.WriteString(proj)
	}
	b.WriteString(" +")
	b.WriteString(created)
	b.WriteString(" [")
	b.WriteString(id)
	b.WriteString("]")
	return b.String()
}

// genID returns a fresh 8-char hex ID using crypto/rand.
func genID() string {
	b := make([]byte, 4)
	if _, err := crand.Read(b); err != nil {
		// Fallback to math/rand if crypto unavailable.
		for i := range b {
			b[i] = byte(rand.Intn(256))
		}
	}
	return hex.EncodeToString(b)
}

var taskTitles = []string{
	"review PR", "write docs", "fix bug", "deploy service",
	"team standup", "plan sprint", "update deps", "run tests",
	"read chapter", "take notes", "groceries", "water plants",
	"inbox zero", "backup files", "clear desk", "schedule meeting",
}

func randomTitle() string {
	return taskTitles[rand.Intn(len(taskTitles))]
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0644)
}
