package cmd

import (
	"context"

	"github.com/heyjan/pixxio-cli/internal/config"
	"github.com/heyjan/pixxio-cli/internal/ui"
)

// AuthCmd is the auth command group.
type AuthCmd struct {
	Login AuthLoginCmd `cmd:"" help:"Store credentials for a pixx.io mediaspace."`
}

// AuthLoginCmd saves mediaspace + API key to config.
// The API key comes from the global --api-key flag or PIXX_API_KEY env.
type AuthLoginCmd struct {
	Mediaspace string `help:"Mediaspace domain (e.g. demo.pixx.io)." required:"" env:"PIXX_MEDIASPACE"`
}

func (c *AuthLoginCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)

	if flags.APIKey == "" {
		return usage("required: --api-key (or set PIXX_API_KEY)")
	}

	cfg := config.FromContext(ctx)
	cfg.Mediaspace = c.Mediaspace
	cfg.APIKey = flags.APIKey
	if err := config.Save(cfg); err != nil {
		return err
	}

	u.Out().Successf("Saved credentials for %s", c.Mediaspace)
	return nil
}
