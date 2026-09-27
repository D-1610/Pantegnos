package mobile

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"Pantegnos/internal/buildinfo"
)

// maxPreviewChars caps what is rendered inline; the full payload is one tap
// away through save or share.
const maxPreviewChars = 120_000

// page is the compiled template. A broken template is a programming error, so it
// is compiled once at start-up and covered by a test.
var page = template.Must(
	template.New("page").Funcs(template.FuncMap{
		"t":        t,
		"upper":    strings.ToUpper,
		"canRetry": func(j jobView) bool { return j.CanRetry },
	}).Parse(pageSource),
)

// view is the data handed to the template. It is a flat copy so the renderer
// never touches live application state.
type view struct {
	Locale       string
	RTL          bool
	Logo         template.HTML
	Theme        string
	Dark         bool
	ShowSettings bool
	Version      string
	Jobs         []jobView
	Formats      []formatView
	Password     *passwordView
	JobCount     int
	ActiveJobs   int
	TotalBytes   int64
}

type jobView struct {
	ID       int64
	Name     string
	Ext      string
	Subtitle string
	Status   string
	Tone     string // ok | bad | busy | warn
	Busy     bool
	Module   string
	Err      string
	Expanded bool
	Wrap     bool
	Text     string
	Truncate bool
	Empty    bool
	CanRetry bool
	CanOpen  bool
	RetryKey string
}

type formatView struct {
	Ext  string
	Desc string
}

type passwordView struct {
	Name  string
	Body  string
	Error string
	Raw   string
}

// render produces the complete application markup for the current state.
func render(snap Snapshot, version string) string {
	v := view{
		Locale:       Locale(),
		RTL:          IsRTL(),
		Logo:         Logo,
		Theme:        snap.Settings.Theme,
		Dark:         snap.Dark,
		Version:      buildinfo.Display(version),
		ShowSettings: snap.ShowSettings,
		Formats:      make([]formatView, 0, len(snap.Formats)),
	}
	for _, f := range snap.Formats {
		v.Formats = append(v.Formats, formatView{Ext: f.Ext, Desc: t(f.Desc)})
	}

	for _, job := range snap.Jobs {
		v.JobCount++
		if !job.done() {
			v.ActiveJobs++
		}
		v.TotalBytes += int64(job.Size)
		v.Jobs = append(v.Jobs, jobToView(job))
	}

	if snap.PasswordJob != nil {
		pv := &passwordView{
			Name: snap.PasswordJob.Name,
			Body: t("password_body", snap.PasswordJob.Name),
		}
		if snap.PasswordErr != "" {
			pv.Error = t("password_rejected")
			pv.Raw = snap.PasswordErr
		}
		v.Password = pv
	}

	var buf bytes.Buffer
	if err := page.Execute(&buf, v); err != nil {
		return "<pre>" + template.HTMLEscapeString(err.Error()) + "</pre>"
	}
	return buf.String()
}

func jobToView(job *Job) jobView {
	jv := jobView{
		ID:       job.ID,
		Name:     job.Name,
		Ext:      job.ext(),
		Status:   t(job.Phase.key()),
		Busy:     job.Phase.busy(),
		Expanded: job.Expanded && job.Phase == PhaseDone,
		Wrap:     job.Wrap,
		Err:      job.Err,
		Module:   job.Module,
		CanRetry: job.Phase == PhaseFailed || job.Phase == PhaseWaitPassword,
		CanOpen:  job.ApkAuthor != "",
		RetryKey: "action_retry",
	}
	if job.Phase == PhaseWaitPassword {
		jv.RetryKey = "action_enter_password"
	}

	switch job.Phase {
	case PhaseDone:
		jv.Tone = "ok"
	case PhaseFailed:
		jv.Tone = "bad"
	case PhaseWaitPassword:
		jv.Tone = "warn"
	default:
		jv.Tone = "busy"
	}

	parts := make([]string, 0, 3)
	if job.FromClip {
		parts = append(parts, t("action_paste"))
	}
	if job.Size > 0 {
		parts = append(parts, humanSize(int64(job.Size)))
	}
	if job.Module != "" {
		parts = append(parts, job.Module)
	}
	jv.Subtitle = strings.Join(parts, " · ")

	if job.Phase == PhaseDone {
		text := job.Text
		if len(text) > maxPreviewChars {
			text = text[:maxPreviewChars]
			jv.Truncate = true
		}
		jv.Text = text
		jv.Empty = text == ""
	}
	return jv
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for size := n / unit; size >= unit && exp < 3; size /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// AppCSS exposes the stylesheet to tools/dumppage, which renders the interface
// outside the app so the layout can be inspected in a desktop browser.
func AppCSS() string { return appCSS }

// MarkupForTools renders the empty application for the same tooling.
func MarkupForTools() string {
	return render(Snapshot{Settings: Settings{Theme: "system"}, Dark: true}, "dev")
}
