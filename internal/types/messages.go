package types

import "time"

type ScreenMode string

const (
	ModeChat    ScreenMode = "chat"
	ModeOptions ScreenMode = "options"
)

type TokenMsg string

type StreamEndMsg struct{}

type StreamErrMsg struct{ Err error }

func (e StreamErrMsg) Error() string { return e.Err.Error() }

// StatusMsg represents a status message to be displayed in the UI
type StatusMsg struct {
	Message  string
	Duration time.Duration
}
