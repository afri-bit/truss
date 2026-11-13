package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/afri-bit/truss/internal/home"
)

// ErrNotRunning means no daemon answers for the home directory.
var ErrNotRunning = errors.New("the TRUSS daemon is not running (start it with: truss daemon start)")

// Client talks to a running daemon.
type Client struct {
	endpoint Endpoint
	http     *http.Client
}

// Dial finds the daemon of a home directory and checks that it answers.
func Dial(ctx context.Context, h home.Dir) (*Client, error) {
	var ep Endpoint
	if err := home.ReadJSON(h.Endpoint(), &ep); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotRunning
		}
		return nil, err
	}
	c := NewClient(ep)
	st, err := c.Status(ctx)
	if err != nil {
		var apiErr *StatusError
		if errors.As(err, &apiErr) && apiErr.Code != http.StatusUnauthorized {
			return nil, err
		}
		// A refused connection or a 401: the endpoint file is stale and the
		// address is free or belongs to something else.
		return nil, ErrNotRunning
	}
	if st.PID != ep.PID {
		return nil, ErrNotRunning // another program answers at the recorded address
	}
	return c, nil
}

// NewClient returns a client for a known endpoint.
func NewClient(ep Endpoint) *Client {
	return &Client{endpoint: ep, http: &http.Client{Timeout: 30 * time.Second}}
}

// Endpoint returns the address and token the client uses.
func (c *Client) Endpoint() Endpoint { return c.endpoint }

// StatusError is a non-2xx answer from the daemon.
type StatusError struct {
	Code    int
	Message string
}

func (e *StatusError) Error() string { return e.Message }

// do sends one request. in and out are JSON bodies and may be nil.
func (c *Client) do(ctx context.Context, method, path string, in, out any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://"+c.endpoint.Address+Prefix+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.endpoint.Token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reach the daemon: %w", err)
	}
	if resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		var e Error
		if json.NewDecoder(resp.Body).Decode(&e) != nil || e.Message == "" {
			e.Message = "the daemon answered " + resp.Status
		}
		return nil, &StatusError{Code: resp.StatusCode, Message: e.Message}
	}
	if out == nil {
		return resp, nil
	}
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("read the daemon's answer: %w", err)
	}
	return resp, nil
}

func (c *Client) call(ctx context.Context, method, path string, in, out any) error {
	resp, err := c.do(ctx, method, path, in, out)
	if err == nil && out == nil {
		_ = resp.Body.Close()
	}
	return err
}

// Status describes the daemon.
func (c *Client) Status(ctx context.Context) (Status, error) {
	var s Status
	return s, c.call(ctx, http.MethodGet, "/status", nil, &s)
}

// Shutdown asks the daemon to stop.
func (c *Client) Shutdown(ctx context.Context) error {
	return c.call(ctx, http.MethodPost, "/shutdown", nil, nil)
}

// Pipelines lists the registered pipelines.
func (c *Client) Pipelines(ctx context.Context) ([]Pipeline, error) {
	var list []Pipeline
	return list, c.call(ctx, http.MethodGet, "/pipelines", nil, &list)
}

// Register registers a definition file.
func (c *Client) Register(ctx context.Context, req RegisterRequest) (Pipeline, error) {
	var p Pipeline
	return p, c.call(ctx, http.MethodPost, "/pipelines", req, &p)
}

// Unregister removes a registration. Its run history stays.
func (c *Client) Unregister(ctx context.Context, name string) error {
	return c.call(ctx, http.MethodDelete, "/pipelines/"+url.PathEscape(name), nil, nil)
}

// SetPaused pauses or resumes the schedules of a pipeline.
func (c *Client) SetPaused(ctx context.Context, name string, paused bool) (Pipeline, error) {
	action := "resume"
	if paused {
		action = "pause"
	}
	var p Pipeline
	return p, c.call(ctx, http.MethodPost, "/pipelines/"+url.PathEscape(name)+"/"+action, nil, &p)
}

// Trigger requests a run of a registered pipeline.
func (c *Client) Trigger(ctx context.Context, name string) (Run, error) {
	var r Run
	return r, c.call(ctx, http.MethodPost, "/pipelines/"+url.PathEscape(name)+"/runs", nil, &r)
}

// Runs lists runs, newest first. An empty pipeline lists all; a limit below
// 1 uses the daemon's default.
func (c *Client) Runs(ctx context.Context, pipeline string, limit int) ([]Run, error) {
	q := url.Values{}
	if pipeline != "" {
		q.Set("pipeline", pipeline)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var list []Run
	return list, c.call(ctx, http.MethodGet, "/runs?"+q.Encode(), nil, &list)
}

// Run returns one run.
func (c *Client) Run(ctx context.Context, id string) (Run, error) {
	var r Run
	return r, c.call(ctx, http.MethodGet, "/runs/"+url.PathEscape(id), nil, &r)
}

// Cancel ends a queued or running run.
func (c *Client) Cancel(ctx context.Context, id string) (Run, error) {
	var r Run
	return r, c.call(ctx, http.MethodPost, "/runs/"+url.PathEscape(id)+"/cancel", nil, &r)
}

// Log returns run-log bytes from offset, the next offset and the run status.
func (c *Client) Log(ctx context.Context, id string, offset int64) ([]byte, int64, string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/runs/"+url.PathEscape(id)+"/log?offset="+strconv.FormatInt(offset, 10), nil, nil)
	if err != nil {
		return nil, 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, "", fmt.Errorf("read the run log: %w", err)
	}
	next, err := strconv.ParseInt(resp.Header.Get(HeaderLogOffset), 10, 64)
	if err != nil {
		next = offset + int64(len(data))
	}
	return data, next, resp.Header.Get(HeaderRunStatus), nil
}

// Browse lists a directory for the register dialog. An empty path means the
// user's home directory.
func (c *Client) Browse(ctx context.Context, path string, hidden bool) (DirListing, error) {
	q := url.Values{}
	if path != "" {
		q.Set("path", path)
	}
	if hidden {
		q.Set("hidden", "true")
	}
	var l DirListing
	return l, c.call(ctx, http.MethodGet, "/fs?"+q.Encode(), nil, &l)
}

// CreateRun opens a tracked run in state running.
func (c *Client) CreateRun(ctx context.Context, req CreateRunRequest) (Run, error) {
	var r Run
	return r, c.call(ctx, http.MethodPost, "/runs", req, &r)
}

// ReportStep reports the state of one step of a tracked run and returns the run.
func (c *Client) ReportStep(ctx context.Context, id string, u StepUpdate) (Run, error) {
	var r Run
	return r, c.call(ctx, http.MethodPost, "/runs/"+url.PathEscape(id)+"/steps", u, &r)
}

// AppendLog appends data to the log of a tracked run at offset, which must be
// the current end of the log. It returns the next offset and the run's status;
// an empty append only reads the status.
func (c *Client) AppendLog(ctx context.Context, id string, offset int64, data []byte) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://"+c.endpoint.Address+Prefix+"/runs/"+url.PathEscape(id)+"/log?offset="+strconv.FormatInt(offset, 10), bytes.NewReader(data))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.endpoint.Token)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("reach the daemon: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		var e Error
		if json.NewDecoder(resp.Body).Decode(&e) != nil || e.Message == "" {
			e.Message = "the daemon answered " + resp.Status
		}
		return 0, "", &StatusError{Code: resp.StatusCode, Message: e.Message}
	}
	next, err := strconv.ParseInt(resp.Header.Get(HeaderLogOffset), 10, 64)
	if err != nil {
		next = offset + int64(len(data))
	}
	return next, resp.Header.Get(HeaderRunStatus), nil
}

// FinishRun closes a tracked run.
func (c *Client) FinishRun(ctx context.Context, id string, req FinishRequest) (Run, error) {
	var r Run
	return r, c.call(ctx, http.MethodPost, "/runs/"+url.PathEscape(id)+"/finish", req, &r)
}
