package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- helpers ----------------------------------------------------------------

func featureJSON(id int64, title string, done int) string {
	return `{"id":` + itoa(id) + `,"title":"` + title + `","project_name":null,"is_done":` + itoa(int64(done)) + `,"completed_at":null,"created_at":"2026-05-01"}`
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	return string([]byte{byte('0' + n)}) // only safe for single digits in tests
}

// more robust int formatting used in test server handlers
func jsonInt(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// --- GetToday ---------------------------------------------------------------

func TestGetToday_happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/today" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("missing Accept header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TodayResponse{
			Pending: []Feature{{ID: 1, Title: "buy milk", IsDone: 0, CreatedAt: "2026-05-01"}},
			Done:    []Feature{{ID: 2, Title: "done task", IsDone: 1, CreatedAt: "2026-05-01"}},
			DoneToday:  1,
			TotalToday: 2,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	resp, err := c.GetToday()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Pending) != 1 {
		t.Errorf("pending: got %d want 1", len(resp.Pending))
	}
	if resp.DoneToday != 1 {
		t.Errorf("done_today: got %d want 1", resp.DoneToday)
	}
}

func TestGetToday_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Detail: "internal error"})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.GetToday()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Create -----------------------------------------------------------------

func TestCreate_happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("text") != "buy eggs @life" {
			t.Errorf("text: got %q", r.FormValue("text"))
		}
		proj := "life"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"feature": Feature{ID: 3, Title: "buy eggs", ProjectName: &proj, IsDone: 0, CreatedAt: "2026-05-01"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	f, err := c.Create("buy eggs @life", "2026-05-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.ID != 3 {
		t.Errorf("id: got %d want 3", f.ID)
	}
}

func TestCreate_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Detail: "bad request"})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.Create("", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- MarkDone ---------------------------------------------------------------

func TestMarkDone_happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/features/5/done" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"feature": Feature{ID: 5, Title: "task", IsDone: 1, CreatedAt: "2026-05-01"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	f, err := c.MarkDone(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.IsDone != 1 {
		t.Errorf("is_done: got %d want 1", f.IsDone)
	}
}

func TestMarkDone_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errorResponse{Detail: "not found"})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.MarkDone(99)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Undone -----------------------------------------------------------------

func TestUndone_happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/features/7/undone" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"feature": Feature{ID: 7, Title: "task", IsDone: 0, CreatedAt: "2026-05-01"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	f, err := c.Undone(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.IsDone != 0 {
		t.Errorf("is_done: got %d want 0", f.IsDone)
	}
}

func TestUndone_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errorResponse{Detail: "not found"})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.Undone(99)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Delete -----------------------------------------------------------------

func TestDelete_happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/features/9" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	if err := c.Delete(9); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errorResponse{Detail: "not found"})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	if err := c.Delete(99); err == nil {
		t.Fatal("expected error")
	}
}

// --- Update -----------------------------------------------------------------

func TestUpdate_happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT got %s", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		// project is merged into title as "@project" suffix (🔴-3 fix)
		if body["title"] != "updated title @work" {
			t.Errorf("title: got %v want %q", body["title"], "updated title @work")
		}
		if _, ok := body["project_name"]; ok {
			t.Error("project_name should not be sent to server")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"feature": Feature{ID: 11, Title: "updated title", IsDone: 0, CreatedAt: "2026-05-01"},
		})
	}))
	defer srv.Close()

	date := "2026-05-01"
	c := New(srv.URL, "")
	f, err := c.Update(11, "updated title", "work", &date)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Title != "updated title" {
		t.Errorf("title: got %q", f.Title)
	}
}

// TestUpdate_noProject verifies that an empty project doesn't add a trailing "@".
func TestUpdate_noProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "just a title" {
			t.Errorf("title: got %v want %q", body["title"], "just a title")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"feature": Feature{ID: 1, Title: "just a title", IsDone: 0, CreatedAt: "2026-05-01"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.Update(1, "just a title", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCreate_serverReturns200 verifies the client accepts HTTP 200 from POST /features
// (the actual server returns 200, not 201 — 🔴-1 fix).
func TestCreate_serverReturns200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proj := "life"
		w.Header().Set("Content-Type", "application/json")
		// 200 instead of 201 — mirrors actual server behaviour
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"feature": Feature{ID: 4, Title: "task", ProjectName: &proj, IsDone: 0, CreatedAt: "2026-05-01"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	f, err := c.Create("task @life", "")
	if err != nil {
		t.Fatalf("unexpected error on HTTP 200: %v", err)
	}
	if f.ID != 4 {
		t.Errorf("id: got %d want 4", f.ID)
	}
}

// TestDelete_serverReturns200WithBody verifies the client accepts HTTP 200 + JSON body
// from DELETE /features/{id} (🔴-2 fix).
func TestDelete_serverReturns200WithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deleted": 9,
			"stats":   map[string]int{"pending": 2, "done": 1},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	if err := c.Delete(9); err != nil {
		t.Fatalf("unexpected error on HTTP 200: %v", err)
	}
}

func TestUpdate_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errorResponse{Detail: "not found"})
	}))
	defer srv.Close()

	date := "2026-05-01"
	c := New(srv.URL, "")
	_, err := c.Update(99, "title", "", &date)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Token header -----------------------------------------------------------

func TestTokenHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Tick-Token") != "secret" {
			t.Errorf("expected token header, got %q", r.Header.Get("X-Tick-Token"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TodayResponse{})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	_, err := c.GetToday()
	if err != nil {
		t.Fatal(err)
	}
}
