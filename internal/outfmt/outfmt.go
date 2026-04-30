package outfmt

import (
	"context"
	"encoding/json"
	"io"
)

type ctxKey struct{}

// Mode represents the output format.
type Mode int

const (
	ModeInteractive Mode = iota
	ModeJSON
	ModePlain
)

// WithMode stores the output mode in context.
func WithMode(ctx context.Context, m Mode) context.Context {
	return context.WithValue(ctx, ctxKey{}, m)
}

func mode(ctx context.Context) Mode {
	m, _ := ctx.Value(ctxKey{}).(Mode)
	return m
}

// IsJSON returns true if JSON output is active.
func IsJSON(ctx context.Context) bool { return mode(ctx) == ModeJSON }

// IsPlain returns true if plain/TSV output is active.
func IsPlain(ctx context.Context) bool { return mode(ctx) == ModePlain }

// WriteJSON encodes v as indented JSON to w.
func WriteJSON(ctx context.Context, w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
