package store

// Feature is a single task entry. ID is a short random hex string (8 chars),
// generated client-side. Random IDs avoid collisions when the same tasks.md
// is edited concurrently across devices (Mac CLI + mobile Obsidian plugin).
type Feature struct {
	ID          string  // 8-char hex (e.g. "a3k7m2x9")
	Title       string
	ProjectName *string
	IsDone      int     // 0 / 1
	CompletedAt *string // YYYY-MM-DD; nil when not done
	CreatedAt   string  // YYYY-MM-DD
}

// TodayResponse is what the TUI asks for on every refresh.
type TodayResponse struct {
	Pending       []Feature
	Done          []Feature // only today's completions
	DoneYesterday []Feature // yesterday's completions (shown below today's done section)
	DoneToday     int
	TotalToday    int
}

// ProjectItem is one project name (for ghost-text autocomplete).
type ProjectItem struct {
	ID   int64
	Name string
}
