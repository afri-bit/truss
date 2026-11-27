// Package notify tells people how a flow's run ended (spec 030): a desktop
// notification through the operating system's own tool, a webhook, or a
// command the flow file names. It never changes a run's result and never
// prints; callers log what it returns.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

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
	// Shell returns the host shell and its arguments for a run string.
	Shell func(run string) (string, []string)
	// Dir is where a notify run command starts.
	Dir string
	// Env is the environment of a notify run command.
	Env []string
}

// Timeout bounds each channel of one notification.
const Timeout = 15 * time.Second

// Send delivers the outcome on every channel the block names and returns
// one error per channel that failed. It does not retry more than once and
// never takes longer than about twice Timeout.
func (s Sender) Send(ctx context.Context, n *pipeline.Notify, o Outcome) []error {
	var errs []error
	if n.Desktop {
		if err := s.desktop(ctx, o); err != nil {
			errs = append(errs, fmt.Errorf("desktop: %w", err))
		}
	}
	if n.Webhook != nil {
		if err := s.webhook(ctx, *n.Webhook, o); err != nil {
			errs = append(errs, fmt.Errorf("webhook: %w", err))
		}
	}
	if n.Run != "" {
		if err := s.command(ctx, n.Run, o); err != nil {
			errs = append(errs, fmt.Errorf("run: %w", err))
		}
	}
	return errs
}

func (s Sender) cmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	if s.Command != nil {
		return s.Command(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...) // #nosec G204 -- fixed tools; texts are arguments, never code
}

// desktop shows a notification with the system's own tool. The title and
// text are passed as arguments or environment, never as code.
func (s Sender) desktop(ctx context.Context, o Outcome) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	var c *exec.Cmd
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		c = s.cmd(ctx, "notify-send", "--app-name=TRUSS", "--", Title(o), Text(o))
	case "darwin":
		c = s.cmd(ctx, "osascript", "-e", "on run argv", "-e",
			"display notification (item 2 of argv) with title (item 1 of argv)", "-e", "end run", Title(o), Text(o))
	case "windows":
		c = s.cmd(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", windowsToast)
		c.Env = append(os.Environ(), "TRUSS_NOTIFY_TITLE="+Title(o), "TRUSS_NOTIFY_TEXT="+Text(o))
	default:
		return fmt.Errorf("no desktop notifications on %s", runtime.GOOS)
	}
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", c.Path, err, firstLine(out))
	}
	return nil
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
	case "slack", "teams":
		return json.Marshal(map[string]string{"text": line})
	case "discord":
		return json.Marshal(map[string]string{"content": line})
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
			return fmt.Errorf("the secret %s is not set (truss secret set %s)", name, name)
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
		client = &http.Client{Timeout: Timeout}
	}
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
		last = fmt.Errorf("the webhook answered %d", resp.StatusCode)
		if resp.StatusCode < 500 {
			return last // a client error does not get better on retry
		}
	}
	return last
}

// command runs the notify run string in the host shell with the outcome in
// TRUSS_NOTIFY_* variables.
func (s Sender) command(ctx context.Context, run string, o Outcome) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	shell, args := s.Shell(run)
	c := s.cmd(ctx, shell, args...)
	c.Dir = s.Dir
	c.Env = append(append([]string(nil), s.Env...),
		"TRUSS_NOTIFY_FLOW="+o.Flow, "TRUSS_NOTIFY_RUN_ID="+o.RunID, "TRUSS_NOTIFY_STATUS="+o.Status,
		"TRUSS_NOTIFY_REASON="+o.Reason, "TRUSS_NOTIFY_TRIGGER="+o.Trigger,
		"TRUSS_NOTIFY_TITLE="+Title(o), "TRUSS_NOTIFY_TEXT="+Text(o))
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, firstLine(out))
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
