package api

// Feature mirrors the server-side Feature struct.
type Feature struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	ProjectName *string `json:"project_name"`
	IsDone      int     `json:"is_done"`
	CompletedAt *string `json:"completed_at"`
	CreatedAt   string  `json:"created_at"`
}

// TodayResponse is the shape of GET /api/today.
type TodayResponse struct {
	Pending   []Feature `json:"pending"`
	Done      []Feature `json:"done"`
	DoneToday int       `json:"done_today"`
	TotalToday int      `json:"total_today"`
}

// ProjectItem is one entry from GET /api/projects.
type ProjectItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// errorResponse is what the server sends on 4xx/5xx.
type errorResponse struct {
	Detail string `json:"detail"`
}
