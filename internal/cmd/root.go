package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/heyjan/pixxio-cli/internal/config"
	"github.com/heyjan/pixxio-cli/internal/errfmt"
	"github.com/heyjan/pixxio-cli/internal/outfmt"
	"github.com/heyjan/pixxio-cli/internal/ui"
)

// Build info, set via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// RootFlags are global flags available to all commands.
type RootFlags struct {
	Color  string `help:"Color output: auto|always|never." default:"auto" env:"PIXX_COLOR"`
	JSON   bool   `help:"Output JSON." short:"j" env:"PIXX_JSON"`
	Plain  bool   `help:"Output stable text (TSV)." short:"p"`
	APIKey string `help:"API key (overrides config, for CI)." name:"api-key" env:"PIXX_API_KEY"`

	Verbose bool `help:"Enable verbose logging." short:"v"`
	DryRun  bool `help:"Preview changes without executing." short:"n" aliases:"noop,preview"`
	Force   bool `help:"Skip confirmations." short:"y" aliases:"yes"`
}

// CLI is the top-level command structure.
type CLI struct {
	RootFlags `embed:""`

	Auth        AuthCmd        `cmd:"" help:"Authenticate with pixx.io."`
	Files       FilesCmd       `cmd:"" help:"Manage files."`
	Collections CollectionsCmd `cmd:"" help:"Manage collections."`
	Directories DirectoriesCmd `cmd:"" help:"Browse directories."`
	Version     VersionCmd     `cmd:"" help:"Print version information."`
}

// VersionCmd prints version info.
type VersionCmd struct{}

func (c *VersionCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, map[string]string{
			"version": version, "commit": commit, "date": date,
		})
	}
	u.Out().Printf("pixx %s (%s, %s)", version, commit, date)
	return nil
}

// Execute parses args and runs the matched command.
func Execute(args []string) error {
	var cli CLI

	cfg, err := config.Load()
	if err != nil {
		slog.Warn("config load failed, using defaults", "err", err)
		cfg = &config.Config{}
	}

	parser, err := kong.New(&cli,
		kong.Name("pixx"),
		kong.Description("pixx.io CLI — manage your digital assets from the terminal."),
		kong.UsageOnError(),
	)
	if err != nil {
		return err
	}

	kctx, err := parser.Parse(args)
	if err != nil {
		return &ExitError{Code: exitUsage, Err: err}
	}

	// Set up logging.
	level := slog.LevelWarn
	if cli.Verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Set up UI.
	u, err := ui.New(ui.Options{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Color:  cli.Color,
	})
	if err != nil {
		return err
	}

	// Set up output mode.
	mode := outfmt.ModeInteractive
	if cli.JSON {
		mode = outfmt.ModeJSON
	} else if cli.Plain {
		mode = outfmt.ModePlain
	}

	// Wire context.
	ctx := context.Background()
	ctx = ui.WithUI(ctx, u)
	ctx = outfmt.WithMode(ctx, mode)

	// --api-key flag overrides config (for CI/scripts).
	if cli.APIKey != "" {
		cfg.APIKey = cli.APIKey
	}
	ctx = config.WithConfig(ctx, cfg)

	// Bind dependencies for Kong.
	kctx.BindTo(ctx, (*context.Context)(nil))
	kctx.Bind(&cli.RootFlags)

	// Run the matched command.
	if err := kctx.Run(ctx, &cli.RootFlags); err != nil {
		u.Err().Errorf("%s", errfmt.Format(err))
		return err
	}
	return nil
}
