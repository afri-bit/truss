package daemon

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/registry"
	"github.com/afri-bit/truss/internal/runs"
	"github.com/afri-bit/truss/internal/webui"
)

// defaultRunLimit bounds a run listing that names no limit.
const defaultRunLimit = 100

// handler returns the daemon's HTTP surface: the API under /api/v1 and
// TRUSS Desktop everywhere else.
//
// Two checks protect the API, because any local web page can send requests
// to a loopback port. Every API call must carry the token from the endpoint
// file, which only the user can read; and the Host header must name this
// machine, which defeats DNS rebinding.
func (d *Daemon) handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h func(http.ResponseWriter, *http.Request) error) {
		method, path, _ := strings.Cut(pattern, " ")
		mux.HandleFunc(method+" "+api.Prefix+path, func(w http.ResponseWriter, r *http.Request) {
			if !d.authorized(r) {
				writeError(w, http.StatusUnauthorized, "missing or wrong access token")
				return
			}
			if err := h(w, r); err != nil {
				writeError(w, statusFor(err), err.Error())
			}
		})
	}
	route("GET /status", d.getStatus)
	route("GET /metrics", d.getMetrics)
	route("POST /shutdown", d.postShutdown)
	route("GET /pipelines", d.listPipelines)
	route("POST /pipelines", d.registerPipeline)
	route("GET /pipelines/{name}", d.getPipeline)
	route("DELETE /pipelines/{name}", d.unregisterPipeline)
	route("POST /pipelines/{name}/pause", d.pausePipeline(true))
	route("POST /pipelines/{name}/resume", d.pausePipeline(false))
	route("POST /pipelines/{name}/runs", d.triggerRun)
	route("GET /fs", d.browse)
	route("GET /runs", d.listRuns)
	route("GET /runs/{id}", d.getRun)
	route("POST /runs/{id}/cancel", d.cancelRun)
	route("GET /runs/{id}/log", d.getRunLog)
	route("POST /runs", d.createTrackedRun)
	route("POST /runs/{id}/steps", d.reportStep)
	route("POST /runs/{id}/log", d.appendRunLog)
	route("POST /runs/{id}/finish", d.finishTrackedRun)
	mux.HandleFunc(api.Prefix+"/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "no such API endpoint")
	})
	mux.Handle("/", http.FileServerFS(webui.Files()))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localHost(r.Host) {
			writeError(w, http.StatusForbidden, "this daemon only answers requests addressed to this machine")
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func (d *Daemon) authorized(r *http.Request) bool {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(token), []byte(d.endpoint.Token)) == 1
}

func localHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// statusFor maps an error to an HTTP status.
func statusFor(err error) int {
	var invalid *invalidError
	var bad *badRequestError
	var coded *statusError
	var tooBig *http.MaxBytesError
	switch {
	case errors.Is(err, registry.ErrNotFound), errors.Is(err, runs.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, registry.ErrExists):
		return http.StatusConflict
	case errors.Is(err, runs.ErrClosed):
		return http.StatusServiceUnavailable
	case errors.Is(err, runs.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, runs.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, runs.ErrTooLarge), errors.As(err, &tooBig):
		return http.StatusRequestEntityTooLarge
	case errors.As(err, &invalid):
		return http.StatusUnprocessableEntity
	case errors.As(err, &bad):
		return http.StatusBadRequest
	case errors.As(err, &coded):
		return coded.code
	}
	return http.StatusInternalServerError
}

type badRequestError struct{ msg string }

func (e *badRequestError) Error() string { return e.msg }

func badRequest(format string, args ...any) error {
	return &badRequestError{fmt.Sprintf(format, args...)}
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	_ = writeJSON(w, status, api.Error{Message: msg})
}

// getMetrics returns the latest resource sample (spec 015).
func (d *Daemon) getMetrics(w http.ResponseWriter, _ *http.Request) error {
	return writeJSON(w, http.StatusOK, d.metrics.Snapshot())
}

func (d *Daemon) getStatus(w http.ResponseWriter, _ *http.Request) error {
	running, queued := d.runs.Counts()
	return writeJSON(w, http.StatusOK, api.Status{
		Version:         d.opts.Version,
		PID:             d.endpoint.PID,
		StartedAt:       d.endpoint.StartedAt,
		Home:            string(d.opts.Home),
		Address:         d.endpoint.Address,
		Pipelines:       len(d.registry.List()),
		RunningRuns:     running,
		QueuedRuns:      queued,
		MaxParallelRuns: d.runs.MaxParallel(),
	})
}

func (d *Daemon) postShutdown(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(http.StatusNoContent)
	d.Stop()
	return nil
}

func (d *Daemon) listPipelines(w http.ResponseWriter, _ *http.Request) error {
	list := []api.Pipeline{}
	shown := d.names()
	for _, e := range d.registry.List() {
		list = append(list, d.viewAs(e, shown))
	}
	return writeJSON(w, http.StatusOK, list)
}

func (d *Daemon) registerPipeline(w http.ResponseWriter, r *http.Request) error {
	var req api.RegisterRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return badRequest("read request: %s", err.Error())
	}
	if !filepath.IsAbs(req.Path) {
		return badRequest("path must be absolute, got %q", req.Path)
	}
	path := filepath.Clean(req.Path)
	def, _, err := readDefinition(path)
	if err != nil {
		return &invalidError{err}
	}
	name := req.Name
	if name == "" {
		name = def.Name
		if registry.CheckName(name) != nil {
			return badRequest("pipeline name %q cannot be used as a registration name; choose one with --name", name)
		}
	} else if err := registry.CheckName(name); err != nil {
		return badRequest("%s", err.Error())
	}
	// A name that already answers to this file is that registration, shown
	// under a name the file gave it; any other holder of the name is a clash.
	if held, ok := d.lookup(name); ok {
		if held.Path != path {
			return fmt.Errorf("%w: %q is %s", registry.ErrExists, name, held.Path)
		}
		return writeJSON(w, http.StatusCreated, d.view(held))
	}
	add := d.registry.Add
	if req.Name != "" {
		add = d.registry.AddChosen
	}
	e, err := add(name, path, d.opts.Now())
	if err != nil {
		return err
	}
	d.refresh(d.opts.Now())
	d.logf("pipeline %s: registered %s", e.Name, e.Path)
	return writeJSON(w, http.StatusCreated, d.view(e))
}

func (d *Daemon) entry(r *http.Request) (registry.Entry, error) {
	return d.find(r.PathValue("name"))
}

func (d *Daemon) getPipeline(w http.ResponseWriter, r *http.Request) error {
	e, err := d.entry(r)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d.view(e))
}

func (d *Daemon) unregisterPipeline(w http.ResponseWriter, r *http.Request) error {
	e, err := d.entry(r)
	if err != nil {
		return err
	}
	if err := d.registry.Remove(e.Name); err != nil {
		return err
	}
	d.refresh(d.opts.Now())
	d.logf("pipeline %s: unregistered", e.Name)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (d *Daemon) pausePipeline(paused bool) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		held, err := d.entry(r)
		if err != nil {
			return err
		}
		e, err := d.registry.SetPaused(held.Name, paused)
		if err != nil {
			return err
		}
		// A resumed schedule counts from now: times that passed while it was
		// paused are not missed runs.
		d.refresh(d.opts.Now())
		d.logf("pipeline %s: paused=%t", e.Name, paused)
		return writeJSON(w, http.StatusOK, d.view(e))
	}
}

func (d *Daemon) triggerRun(w http.ResponseWriter, r *http.Request) error {
	e, err := d.entry(r)
	if err != nil {
		return err
	}
	run, err := d.submit(e.Name, api.TriggerManual, time.Time{})
	if err != nil {
		return err
	}
	d.logf("pipeline %s: manual trigger -> run %s (%s)", e.Name, run.ID, run.Status)
	return writeJSON(w, http.StatusAccepted, showRun(d.names(), run))
}

func (d *Daemon) listRuns(w http.ResponseWriter, r *http.Request) error {
	limit := defaultRunLimit
	if text := r.URL.Query().Get("limit"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 {
			return badRequest("limit must be a whole number of at least 1")
		}
		limit = n
	}
	var names []string
	if name := r.URL.Query().Get("pipeline"); name != "" {
		names = d.historyNames(name)
	}
	shown := d.names()
	list := d.runs.ListFor(names, limit)
	for i := range list {
		list[i] = showRun(shown, list[i])
	}
	return writeJSON(w, http.StatusOK, list)
}

func (d *Daemon) getRun(w http.ResponseWriter, r *http.Request) error {
	run, err := d.runs.Get(r.PathValue("id"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, showRun(d.names(), run))
}

func (d *Daemon) cancelRun(w http.ResponseWriter, r *http.Request) error {
	run, err := d.runs.Cancel(r.PathValue("id"))
	if err != nil {
		return err
	}
	d.logf("run %s: cancel requested", run.ID)
	return writeJSON(w, http.StatusOK, showRun(d.names(), run))
}

func (d *Daemon) getRunLog(w http.ResponseWriter, r *http.Request) error {
	var offset int64
	if text := r.URL.Query().Get("offset"); text != "" {
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil || n < 0 {
			return badRequest("offset must be a whole number of at least 0")
		}
		offset = n
	}
	data, next, status, err := d.runs.Log(r.PathValue("id"), offset)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set(api.HeaderLogOffset, strconv.FormatInt(next, 10))
	w.Header().Set(api.HeaderRunStatus, status)
	// The log is served as plain text with nosniff, so a browser never renders it.
	_, err = w.Write(data) // #nosec G705
	return err
}

// maxJSONBody bounds the JSON bodies of the tracking endpoints. A run of the
// largest allowed size (api.MaxTrackedSteps steps) fits well within it.
const maxJSONBody = 1 << 20

// readJSON decodes a request body into v. A body over maxJSONBody is a 413
// and an unreadable one a 400.
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return err
		}
		return badRequest("read request: %s", err.Error())
	}
	return nil
}

func (d *Daemon) createTrackedRun(w http.ResponseWriter, r *http.Request) error {
	var req api.CreateRunRequest
	if err := readJSON(w, r, &req); err != nil {
		return err
	}
	run, err := d.runs.Track(req.Pipeline, req.Steps)
	if err != nil {
		return err
	}
	d.logf("pipeline %s: tracked local run %s started", run.Pipeline, run.ID)
	return writeJSON(w, http.StatusCreated, showRun(d.names(), run))
}

func (d *Daemon) reportStep(w http.ResponseWriter, r *http.Request) error {
	var u api.StepUpdate
	if err := readJSON(w, r, &u); err != nil {
		return err
	}
	run, err := d.runs.TrackStep(r.PathValue("id"), u)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, showRun(d.names(), run))
}

func (d *Daemon) appendRunLog(w http.ResponseWriter, r *http.Request) error {
	offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		return badRequest("offset must be a whole number of at least 0")
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, api.MaxLogChunk))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return err
		}
		return badRequest("read request: %s", err.Error())
	}
	next, status, err := d.runs.TrackLog(r.PathValue("id"), offset, data)
	if err != nil {
		return err
	}
	w.Header().Set(api.HeaderLogOffset, strconv.FormatInt(next, 10))
	w.Header().Set(api.HeaderRunStatus, status)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (d *Daemon) finishTrackedRun(w http.ResponseWriter, r *http.Request) error {
	var req api.FinishRequest
	if err := readJSON(w, r, &req); err != nil {
		return err
	}
	run, err := d.runs.TrackFinish(r.PathValue("id"), req.Status, req.Reason)
	if err != nil {
		return err
	}
	d.logf("run %s: tracked local run %s", run.ID, run.Status)
	return writeJSON(w, http.StatusOK, showRun(d.names(), run))
}
