package mobile

import "strings"

// Phase is the lifecycle of a single config inside the queue.
type Phase int

const (
	PhaseQueued Phase = iota
	PhaseInspecting
	PhaseWaitPassword
	PhaseWorking
	PhaseDone
	PhaseFailed
)

func (p Phase) key() string {
	switch p {
	case PhaseQueued, PhaseInspecting:
		return "state_reading"
	case PhaseWaitPassword:
		return "state_waiting_password"
	case PhaseWorking:
		return "state_working"
	case PhaseDone:
		return "state_done"
	default:
		return "state_failed"
	}
}

func (p Phase) busy() bool {
	return p == PhaseQueued || p == PhaseInspecting || p == PhaseWorking
}

func (p Phase) finished() bool { return p == PhaseDone || p == PhaseFailed }

// Job is one config file on its way through the decryptor.
type Job struct {
	ID        int64
	Name      string
	Size      int
	FromClip  bool
	Phase     Phase
	Module    string
	ApkAuthor string
	Text      string
	OutName   string
	Err       string
	Expanded  bool
	Wrap      bool

	// promptPassword forces the password sheet even when the module did not
	// advertise one, which is how "enter the password again" works.
	promptPassword bool
	// data is the raw file, retained only while a retry is still possible.
	data []byte
	// lastError is shown under the password prompt after a rejected attempt.
	lastError string
}

func (j *Job) done() bool { return j.Phase.finished() }

// ext is the badge shown on the card, e.g. NPVS.
func (j *Job) ext() string {
	raw := j.Name
	if i := strings.LastIndex(raw, "."); i >= 0 && i < len(raw)-1 {
		raw = raw[i+1:]
	} else {
		return ""
	}
	raw = strings.ToUpper(raw)
	if len(raw) > 4 {
		raw = raw[:4]
	}
	return raw
}

// Settings are the few preferences the user can change from the sheet.
type Settings struct {
	Theme  string `json:"theme"`  // system | light | dark
	Locale string `json:"locale"` // en | fa
}

func defaultSettings() Settings {
	return Settings{Theme: "system", Locale: "en"}
}

// FormatEntry documents one supported extension in the settings sheet.
type FormatEntry struct {
	Ext  string
	Desc string
}
