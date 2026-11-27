package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/engine"
	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/notify"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/runlog"
)

// runSpec says what one local run does beyond running the whole flow.
type runSpec struct {
	// flow is the flow as the user wrote it, for the hint after a failure.
	flow string
	// debug marks a run of a slice of the flow: its log is printed and not saved
	// (spec 023).
	debug bool
	// projectDir is the root of the flow's project, for TRUSS_PROJECT_DIR;
	// empty means the run's directory (spec 026).
	projectDir string
}

// runLocal runs a pipeline definition in this process, without the daemon.
// pos holds the flow name or path (spec 016). It writes the run log to stdout
// and to .truss/state/<flow>/run.log. With opts.track it also reports the run
// to the daemon, which must be running (spec 010). With opts.step or
// opts.from it runs only part of the flow and saves no run log (spec 023).
func runLocal(ctx context.Context, pos []string, opts runOptions, cwd string, env []string, stdout, stderr io.Writer) int {
	if len(pos) == 0 {
		printError(stderr, needFlowError(cwd, "run").Error())
		return 1
	}
	file, err := resolveFile(cwd, pos[0])
	if err != nil {
		printError(stderr, err.Error())
		return 1
	}
	l, err := load(ctx, file)
	if err != nil {
		printError(stderr, err.Error())
		return 1
	}
	def := l.def
	spec := runSpec{flow: pos[0], debug: opts.debug(), projectDir: projectRootOf(file.path)}
	if opts.debug() || len(opts.env) > 0 {
		sliced, err := pipeline.Slice(def, opts.selection())
		if err != nil {
			printError(stderr, clean(err.Error()))
			return 1
		}
		if spec.debug {
			ids := make([]string, len(sliced.Steps))
			for i, s := range sliced.Steps {
				ids[i] = clean(s.ID)
			}
			total := len(def.Steps)
			if plan, err := pipeline.Plan(def); err == nil {
				total = len(plan.Graph)
			}
			fmt.Fprintf(stderr, "debug run: %d of %d steps (%s); the run log is not saved\n", len(ids), total, strings.Join(ids, ", "))
		}
		def = sliced
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	var tr *tracker
	if opts.track {
		tr, err = startTracking(ctx, def, env, stderr, cancelRun)
		if err != nil {
			printError(stderr, err.Error())
			return 1
		}
	}
	code, status, reason := execute(ctx, runCtx, def, file.stem, cwd, env, stdout, stderr, tr, spec)
	if tr != nil {
		tr.finish(status, reason)
	}
	if opts.notify {
		// The block of the file as written: a debug run or --env builds a
		// smaller flow without it.
		notifyLocal(ctx, l.def.Name, l.def.Notify, status, reason, cmp.Or(spec.projectDir, filepath.Dir(file.path)), env, stderr)
	}
	return code
}

// notifyLocal sends the flow's notifications for a local run with --notify
// (spec 030). A failure to notify is reported and never changes the exit
// code.
func notifyLocal(ctx context.Context, flow string, n *pipeline.Notify, status, reason, dir string, env []string, stderr io.Writer) {
	if n == nil {
		fmt.Fprintln(stderr, "note: --notify: the flow has no notify block")
		return
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "note: --notify: the run was interrupted; no notification sent")
		return
	}
	o := notify.Outcome{Flow: flow, RunID: "local", Status: status, Reason: clean(reason), Trigger: api.TriggerLocal, Ended: time.Now()}
	if !notify.Wanted(n, o) {
		return
	}
	sender := notify.Sender{Secret: func(name string) (string, bool) {
		all, err := secretLoader(env)()
		if err != nil {
			return "", false
		}
		v, ok := all[name]
		return v, ok
	}, Dir: dir, Env: env}
	// A Ctrl-C while notifying stops it.
	for _, err := range sender.Send(ctx, n, o) {
		fmt.Fprintf(stderr, "note: notification not sent: %s\n", clean(err.Error()))
	}
}

// startTracking opens the run on the daemon of this invocation's TRUSS home.
func startTracking(ctx context.Context, def pipeline.Pipeline, env []string, stderr io.Writer, cancelRun context.CancelFunc) (*tracker, error) {
	plan, err := pipeline.Plan(def)
	if err != nil {
		return nil, err
	}
	h, err := home.Resolve(env)
	if err != nil {
		return nil, err
	}
	c, err := api.Dial(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("--track: %w", err)
	}
	req := api.CreateRunRequest{Pipeline: plan.Name}
	for _, s := range plan.Graph {
		req.Steps = append(req.Steps, api.PlanStep{ID: s.ID, Needs: s.Needs})
	}
	tr, err := newTracker(ctx, c, req, stderr, cancelRun)
	if err != nil {
		return nil, fmt.Errorf("--track: %w", err)
	}
	fmt.Fprintf(stderr, "tracking as run %s (truss logs %s)\n", tr.id, tr.id)
	return tr, nil
}

// execute runs the pipeline and returns the exit code and how the run ended
// (an api run status and reason) for the tracker. ctx is the caller's;
// runCtx also ends when the daemon cancels a tracked run.
func execute(ctx, runCtx context.Context, def pipeline.Pipeline, stem, cwd string, env []string, stdout, stderr io.Writer, tr *tracker, spec runSpec) (int, string, string) {
	// Generated files live in .truss/state/<flow>/ under the working directory;
	// steps get its data folder, the counterpart of the daemon's per-pipeline
	// one (specs/008-workspace-and-cache, specs/016-project-layout).
	abs := workingRoot(cwd)
	st, err := prepareState(abs, stem, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "error: prepare state folder: %s\n", err.Error())
		return 1, api.RunFailed, err.Error()
	}
	env = append(append([]string(nil), env...), "TRUSS_DATA_DIR="+st.Data, "TRUSS_CACHE_DIR="+st.Cache)

	var extra io.Writer
	if tr != nil {
		extra = tr
	}
	// A debug run prints its log and keeps the last real run's saved log.
	var (
		log     *runlog.Log
		tmp     *os.File
		tmpPath string
		dest    string
	)
	if spec.debug {
		log = runlog.New(stdout, time.Now)
	} else if log, tmp, tmpPath, dest, err = openRunLog(st.Dir, stdout, extra); err != nil {
		printError(stderr, err.Error())
		return 1, api.RunFailed, err.Error()
	}
	discard := func() {
		if tmp != nil {
			discardRunLog(tmp, tmpPath)
		}
	}
	commit := func() error {
		if tmp == nil {
			return nil
		}
		return commitRunLog(tmp, tmpPath, dest)
	}
	log.Setup(runlog.OSName(runtime.GOOS), runtime.GOARCH, executor.Shell(), abs)
	var events engine.StepEvents = log
	if tr != nil {
		events = newTrackedEvents(log, tr, runCtx)
	}
	info := engine.RunInfo{ID: localRunID(), Trigger: api.TriggerLocal, ProjectDir: cmp.Or(spec.projectDir, abs)}
	if tr != nil {
		info.ID = tr.id
	}
	run, err := engine.Run(runCtx, def, cwd, env, events, engine.WithSecrets(secretLoader(env)), engine.WithRun(info), engine.WithKillNow(killNowOf(ctx)))
	if err != nil {
		discard()
		return interrupted(ctx, tr, err, stderr)
	}
	if err := ctx.Err(); err != nil {
		// A cancel from the daemon that arrives after the run ended does not
		// change its outcome. FR-017: an interrupted run never saves a run log.
		discard()
		return interrupted(ctx, tr, err, stderr)
	}
	if run.Status != pipeline.StatusSucceeded {
		log.SummaryFailed(len(run.Steps), run.Failure())
		if err := commit(); err != nil {
			printError(stderr, err.Error())
		}
		if run.FailedStep != "" && !spec.debug {
			fmt.Fprintln(stderr, rerunHint(spec.flow, run.FailedStep))
		}
		return 1, api.RunFailed, run.Failure()
	}
	log.SummarySucceeded(len(run.Steps))
	if err := commit(); err != nil {
		printError(stderr, err.Error())
		return 1, api.RunFailed, err.Error()
	}
	return 0, api.RunSucceeded, ""
}

// interrupted reports a run that ended early: the caller interrupted it, the
// daemon cancelled it, or the engine refused it.
func interrupted(ctx context.Context, tr *tracker, err error, stderr io.Writer) (int, string, string) {
	switch {
	case tr != nil && tr.daemonCancelled() && ctx.Err() == nil:
		fmt.Fprintln(stderr, "error: cancelled from the daemon")
		return 1, api.RunCancelled, "cancelled from the daemon"
	case ctx.Err() != nil:
		printError(stderr, err.Error())
		return 1, api.RunCancelled, "interrupted"
	}
	printError(stderr, err.Error())
	return 1, api.RunFailed, err.Error()
}

// openRunLog starts a run log in dir, the pipeline's state folder.
func openRunLog(dir string, stdout, extra io.Writer) (*runlog.Log, *os.File, string, string, error) {
	dest := filepath.Join(dir, "run.log")
	f, tmpPath, err := createTemp(dir, "run.log.", ".tmp")
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("prepare run log: %w", err)
	}
	out := io.MultiWriter(stdout, f)
	if extra != nil {
		out = io.MultiWriter(stdout, f, extra)
	}
	return runlog.New(out, time.Now), f, tmpPath, dest, nil
}

// ensureDir makes sure path is a directory of its own, creating it with mode
// when it is missing (its parent must exist), and reports whether it created
// it. A symbolic link or Windows junction is refused, as is anything that is
// not a directory: a checkout can carry a link anywhere under .truss, and
// TRUSS must not write the run log, data or cache into the folder it points
// to (specs/002-local-run-log FR-018, specs/016-project-layout FR-014).
func ensureDir(path string, mode fs.FileMode) (bool, error) {
	// Create first: Mkdir never follows a link, and finding the folder already
	// there (another run may have just made it) is fine once it checks out.
	err := os.Mkdir(path, mode) // #nosec G301 -- .truss keeps 0755; state uses 0700
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return false, err
	}
	info, err := os.Lstat(path)
	switch {
	case err != nil:
		return false, err
	case info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0:
		return false, fmt.Errorf("%s is a link or other reparse point; TRUSS does not write through links in .truss", path)
	case !info.IsDir():
		return false, fmt.Errorf("%s is not a directory", path)
	}
	return false, nil
}

// createTemp creates a new file prefix<random>suffix in dir. O_EXCL means it
// never opens an existing file, so a symbolic link a checkout planted in .truss
// is not followed. Unlike os.CreateTemp it keeps the usual 0666 mode less the
// umask, so the saved run log has the permissions it always had.
func createTemp(dir, prefix, suffix string) (*os.File, string, error) {
	var lastErr error
	for range 10 {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return nil, "", err
		}
		path := filepath.Join(dir, prefix+hex.EncodeToString(b[:])+suffix)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666) // #nosec G302 G304 -- new file under .truss; mode as os.Create
		if err == nil {
			return f, path, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, "", err
		}
		lastErr = err
	}
	return nil, "", lastErr
}

func commitRunLog(tmp *os.File, tmpPath, dest string) error {
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close run log: %w", err)
	}
	if err := replaceFile(tmpPath, dest); err != nil {
		return fmt.Errorf("replace run log: %w", err)
	}
	return nil
}

// replaceFile moves tmpPath to dest.
// When the host cannot replace an existing dest, the previous file is restored
// if the move fails.
func replaceFile(tmpPath, dest string) error {
	err := os.Rename(tmpPath, dest)
	if err == nil {
		return nil
	}
	// Lstat: a link at dest (even a dangling one) is moved aside, never followed.
	if _, statErr := os.Lstat(dest); statErr != nil {
		return err
	}
	backup := dest + ".bak"
	_ = os.Remove(backup)
	if renameErr := os.Rename(dest, backup); renameErr != nil {
		return err
	}
	if renameErr := os.Rename(tmpPath, dest); renameErr != nil {
		_ = os.Rename(backup, dest)
		return renameErr
	}
	_ = os.Remove(backup)
	return nil
}

func discardRunLog(tmp *os.File, tmpPath string) {
	_ = tmp.Close()
	_ = os.Remove(tmpPath)
}

// localRunID names a local run that the daemon does not know: the time and
// a random suffix, like a daemon run id, with "local-" in front (spec 026).
func localRunID() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return "local-" + time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

type killNowKey struct{}

// withKillNow carries the channel a second Ctrl-C closes.
func withKillNow(ctx context.Context, ch <-chan struct{}) context.Context {
	return context.WithValue(ctx, killNowKey{}, ch)
}

// killNowOf returns the channel of withKillNow, or nil.
func killNowOf(ctx context.Context) <-chan struct{} {
	ch, _ := ctx.Value(killNowKey{}).(<-chan struct{})
	return ch
}
