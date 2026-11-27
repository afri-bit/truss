// Package notify tells people how a flow's run ended (spec 030): a desktop
// notification through the operating system's own tool, a webhook, or a
// command the flow file names. It never changes a run's result and never
// prints; callers log what it returns.
package notify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/pipeline"
)

// Outcome is how a run ended, as a notification needs it.
type Outcome struct {
	Flow    string `json:"flow"`
	RunID   string `json:"run_id"`
	Status  string `json:"status"` // succeeded, failed, cancelled, skipped
	Reason  string `json:"reason,omitempty"`
	Trigger string `json:"trigger"`
	// Previous is the status of the flow's run before this one; "" when
	// there was none.
	Previous string `json:"previous_status,omitempty"`
	// Missed is set for a scheduled time that passed while the daemon was
	// not running.
	Missed bool      `json:"missed,omitempty"`
	Ended  time.Time `json:"finished_at"`
}

// Wanted reports whether the notify block asks to hear about the outcome.
func Wanted(n *pipeline.Notify, o Outcome) bool {
	if n == nil {
		return false
	}
	on := n.On
	if len(on) == 0 {
		on = []string{"failure"}
	}
	bad := o.Status == "failed" || o.Missed
	for _, w := range on {
		switch {
		case w == "always" && (o.Status != "skipped" || o.Missed),
			w == "failure" && bad,
			w == "success" && o.Status == "succeeded",
			w == "recovered" && o.Status == "succeeded" && o.Previous == "failed",
			w == "changed" && o.Previous != "" && o.Status != o.Previous && o.Status != "skipped":
			return true
		}
	}
	return false
}

// Title and Text are the words of a notification.
func Title(o Outcome) string {
	switch {
	case o.Missed:
		return "TRUSS: " + o.Flow + " missed a scheduled run"
	case o.Status == "succeeded" && o.Previous == "failed":
		return "TRUSS: " + o.Flow + " recovered"
	}
	return "TRUSS: " + o.Flow + " " + o.Status
}

// Text is the body of a notification.
func Text(o Outcome) string {
	text := "Run " + o.RunID + " (" + o.Trigger + ")"
	if o.Reason != "" {
		text += ": " + o.Reason
	}
	return text
}

// Sender sends notifications. Its fields may be replaced in tests.
type Sender struct {
	// Secret returns a stored secret's value, for a webhook "secret:NAME".
	Secret func(name string) (string, bool)
	// HTTP posts webhooks.
	HTTP *http.Client
	// Command starts a process; nil means exec.CommandContext.
	Command func(ctx context.Context, name string, args ...string) *exec.Cmd
	// Dir is where a notify run command starts.
	Dir string
	// Env is the environment of a notify run command.
	Env []string
}

// Timeout bounds each channel of one notification, retries included.
var Timeout = 15 * time.Second

// Send delivers the outcome on every channel the block names and returns
// one error per channel that failed. Each channel has its own Timeout, so
// one that hangs does not take the time of the next; Send takes at most
// three times Timeout and returns early when ctx ends.
func (s Sender) Send(ctx context.Context, n *pipeline.Notify, o Outcome) []error {
	var errs []error
	channel := func(name string, send func(context.Context) error) {
		if ctx.Err() != nil {
			errs = append(errs, fmt.Errorf("%s: not sent, stopped", name))
			return
		}
		cctx, cancel := context.WithTimeout(ctx, Timeout)
		defer cancel()
		if err := send(cctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	if n.Desktop {
		channel("desktop", func(c context.Context) error { return s.desktop(c, o) })
	}
	if n.Webhook != nil {
		channel("webhook", func(c context.Context) error { return s.webhook(c, *n.Webhook, o) })
	}
	if n.Run != "" {
		channel("run", func(c context.Context) error { return s.command(c, n.Run, o) })
	}
	return errs
}

func (s Sender) cmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	if s.Command != nil {
		return s.Command(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...) // #nosec G204 G702 -- fixed tools; texts are arguments, never code
}

// desktop shows a notification with the system's own tool. The title and
// text are passed as arguments or environment, never as code.
func (s Sender) desktop(ctx context.Context, o Outcome) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		c = s.cmd(ctx, "notify-send", "--app-name=TRUSS", "--", Title(o), Text(o))
	case "darwin":
		c = s.cmd(ctx, "osascript", "-e", "on run argv", "-e",
			"display notification (item 2 of argv) with title (item 1 of argv)", "-e", "end run", Title(o), Text(o))
	case "windows":
		ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		c = s.cmd(ctx, ps, "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(windowsToast))
		c.Env = append(os.Environ(), "TRUSS_NOTIFY_TITLE="+Title(o), "TRUSS_NOTIFY_TEXT="+Text(o))
	default:
		return fmt.Errorf("no desktop notifications on %s", runtime.GOOS)
	}
	// A tool that leaves a child holding its output must not keep us.
	c.WaitDelay = 2 * time.Second
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", filepath.Base(c.Path), err, firstLine(out))
	}
	return nil
}

// encodePowerShell gives a script to -EncodedCommand: base64 of UTF-16LE,
// so no quoting of the command line can change it.
func encodePowerShell(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, r := range u {
		binary.LittleEndian.PutUint16(b[2*i:], r)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// windowsToast shows a toast with the texts from the environment, so they
// are never parsed as PowerShell.
const windowsToast = `$ErrorActionPreference='Stop'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
$t = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$n = $t.GetElementsByTagName('text')
$n.Item(0).AppendChild($t.CreateTextNode($env:TRUSS_NOTIFY_TITLE)) | Out-Null
$n.Item(1).AppendChild($t.CreateTextNode($env:TRUSS_NOTIFY_TEXT)) | Out-Null
$app = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($app).Show([Windows.UI.Notifications.ToastNotification]::new($t))`

// Body returns the JSON a webhook of the format receives.
func Body(format string, o Outcome) ([]byte, error) {
	line := Title(o) + " - " + Text(o)
	switch format {
	case "slack":
		// Slack reads <...> as links and mentions; escape them.
		return json.Marshal(map[string]string{"text": strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(line)})
	case "teams":
		return json.Marshal(map[string]string{"text": line})
	case "discord":
		// No @everyone or @here from a flow name or a reason.
		return json.Marshal(map[string]any{"content": line, "allowed_mentions": map[string]any{"parse": []string{}}})
	}
	return json.Marshal(o)
}

func (s Sender) webhook(ctx context.Context, w pipeline.Webhook, o Outcome) error {
	target := w.URL
	if name, ok := strings.CutPrefix(target, "secret:"); ok {
		v, found := "", false
		if s.Secret != nil {
			v, found = s.Secret(name)
		}
		if !found {
			return fmt.Errorf("the secret %s is not set or the secret store cannot be read (truss secret list)", name)
		}
		if err := pipeline.CheckWebhookURL(v); err != nil || strings.HasPrefix(v, "secret:") {
			return fmt.Errorf("the secret %s does not hold an https URL", name)
		}
		target = v
	}
	body, err := Body(w.Format, o)
	if err != nil {
		return err
	}
	client := s.HTTP
	if client == nil {
		client = &http.Client{}
	}
	// A redirect could carry the URL, which holds a token, to another host,
	// or the body to plain http: it counts as a failure.
	noRedirect := *client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client = &noRedirect
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
		if err != nil {
			return errors.New("the webhook URL cannot be used") // never quote it: it may hold a token
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "truss")
		resp, err := client.Do(req)
		if err != nil {
			last = errors.New("the webhook could not be reached")
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 300 {
			return nil
		}
		if resp.StatusCode < 400 {
			return fmt.Errorf("the webhook answered %d (a redirect, which TRUSS does not follow)", resp.StatusCode)
		}
		last = fmt.Errorf("the webhook answered %d", resp.StatusCode)
		if resp.StatusCode < 500 {
			return last // a client error does not get better on retry
		}
	}
	return last
}

// command runs the notify run string in the host shell, in a process tree
// of its own that is ended as a whole at the deadline, with the outcome in
// TRUSS_NOTIFY_* variables. Its output goes to no log: it may print what
// its environment holds.
func (s Sender) command(ctx context.Context, run string, o Outcome) error {
	env := append(append([]string(nil), s.Env...),
		"TRUSS_NOTIFY_FLOW="+o.Flow, "TRUSS_NOTIFY_RUN_ID="+o.RunID, "TRUSS_NOTIFY_STATUS="+o.Status,
		"TRUSS_NOTIFY_REASON="+o.Reason, "TRUSS_NOTIFY_TRIGGER="+o.Trigger,
		"TRUSS_NOTIFY_TITLE="+Title(o), "TRUSS_NOTIFY_TEXT="+Text(o))
	p, err := executor.Start(ctx, executor.Request{Run: run, Dir: s.Dir, Env: env, KillTimeout: 2 * time.Second})
	if err != nil {
		return err
	}
	res := p.Wait(ctx)
	switch {
	case ctx.Err() != nil:
		return fmt.Errorf("stopped after %s", Timeout)
	case res.Err != nil:
		return res.Err
	case res.ExitCode != nil && *res.ExitCode != 0:
		return fmt.Errorf("exit %d", *res.ExitCode)
	}
	return nil
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	if len(line) > 200 {
		line = line[:200]
	}
	return line
}
