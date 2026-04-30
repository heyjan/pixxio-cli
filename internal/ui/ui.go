package ui

import (
	"context"
	"fmt"
	"io"

	"github.com/muesli/termenv"
)

type ctxKey struct{}

// Options configures the UI.
type Options struct {
	Stdout io.Writer
	Stderr io.Writer
	Color  string // auto|always|never
}

// UI provides styled terminal output with strict stdout/stderr separation.
type UI struct {
	out *Writer
	err *Writer
}

// Writer wraps an io.Writer with optional color support.
type Writer struct {
	w      io.Writer
	output *termenv.Output
}

// New creates a UI instance.
func New(opts Options) (*UI, error) {
	profile := termenv.ColorProfile()
	if opts.Color == "never" {
		profile = termenv.Ascii
	} else if opts.Color == "always" {
		profile = termenv.ANSI256
	}
	outOutput := termenv.NewOutput(opts.Stdout, termenv.WithProfile(profile))
	errOutput := termenv.NewOutput(opts.Stderr, termenv.WithProfile(profile))
	return &UI{
		out: &Writer{w: opts.Stdout, output: outOutput},
		err: &Writer{w: opts.Stderr, output: errOutput},
	}, nil
}

// Out returns the stdout writer (for data).
func (u *UI) Out() *Writer { return u.out }

// Err returns the stderr writer (for diagnostics).
func (u *UI) Err() *Writer { return u.err }

// Printf writes a formatted line to the writer.
func (w *Writer) Printf(format string, a ...any) {
	fmt.Fprintf(w.w, format+"\n", a...)
}

// Successf writes a green formatted line.
func (w *Writer) Successf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(w.w, w.output.String(msg).Foreground(w.output.Color("2")))
}

// Errorf writes a red formatted line.
func (w *Writer) Errorf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(w.w, w.output.String(msg).Foreground(w.output.Color("1")))
}

// Warnf writes a yellow formatted line.
func (w *Writer) Warnf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(w.w, w.output.String(msg).Foreground(w.output.Color("3")))
}

// WithUI stores the UI in context.
func WithUI(ctx context.Context, u *UI) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// FromContext retrieves the UI from context.
func FromContext(ctx context.Context) *UI {
	u, _ := ctx.Value(ctxKey{}).(*UI)
	return u
}
