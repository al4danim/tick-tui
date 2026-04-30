package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a thin wrapper around the tick HTTP API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// New creates a Client with a 5-second timeout.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetToday fetches today's pending + done features.
func (c *Client) GetToday() (*TodayResponse, error) {
	var resp TodayResponse
	if err := c.get("/api/today", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetProjects fetches all projects (used for ghost-text autocomplete).
func (c *Client) GetProjects() ([]ProjectItem, error) {
	var resp []ProjectItem
	if err := c.get("/api/projects", &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Create posts a new feature. text may contain "@project" suffix.
// date is "YYYY-MM-DD" or "" (server will store NULL).
func (c *Client) Create(text, date string) (*Feature, error) {
	form := url.Values{}
	form.Set("text", text)
	form.Set("completion_date", date)

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/features", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	c.setToken(req)

	respBody, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Feature Feature `json:"feature"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("decode create response: %w", err)
	}
	return &wrapper.Feature, nil
}

// Update PUTs updated fields for an existing feature.
// The server derives project from the "@project" suffix in title, so we
// concatenate project back into the title rather than sending project_name
// (the server ignores project_name on PUT).
// date is a *string: nil = do not send field, "" = send null, "YYYY-MM-DD" = set date.
func (c *Client) Update(id int64, title, project string, date *string) (*Feature, error) {
	fullTitle := strings.TrimSpace(title)
	if p := strings.TrimSpace(project); p != "" {
		fullTitle = fullTitle + " @" + p
	}

	payload := map[string]interface{}{
		"title": fullTitle,
	}
	if date != nil {
		if *date == "" {
			payload["completion_date"] = nil
		} else {
			payload["completion_date"] = *date
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/features/%d", c.baseURL, id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.setToken(req)

	respBody, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Feature Feature `json:"feature"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("decode update response: %w", err)
	}
	return &wrapper.Feature, nil
}

// MarkDone sends PATCH /features/{id}/done.
func (c *Client) MarkDone(id int64) (*Feature, error) {
	return c.patch(fmt.Sprintf("/features/%d/done", id))
}

// Undone sends PATCH /features/{id}/undone.
func (c *Client) Undone(id int64) (*Feature, error) {
	return c.patch(fmt.Sprintf("/features/%d/undone", id))
}

// Delete sends DELETE /features/{id}.
func (c *Client) Delete(id int64) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/features/%d", c.baseURL, id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	c.setToken(req)

	_, err = c.do(req)
	return err
}

// --- helpers ---------------------------------------------------------------

func (c *Client) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	c.setToken(req)

	body, err := c.do(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *Client) patch(path string) (*Feature, error) {
	req, err := http.NewRequest(http.MethodPatch, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	c.setToken(req)

	body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Feature Feature `json:"feature"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("decode patch response: %w", err)
	}
	return &wrapper.Feature, nil
}

// do executes req and returns the response body.
// It accepts any 2xx status code as success, matching the server's actual behaviour
// (e.g. POST /features returns 200, DELETE /features/{id} returns 200 with a body).
func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp errorResponse
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Detail != "" {
			return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, errResp.Detail)
		}
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return body, nil
}

func (c *Client) setToken(req *http.Request) {
	if c.token != "" {
		req.Header.Set("X-Tick-Token", c.token)
	}
}
