package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/heyjan/pixxio-cli/internal/config"
	"github.com/heyjan/pixxio-cli/internal/outfmt"
	"github.com/heyjan/pixxio-cli/internal/pixxapi"
	"github.com/heyjan/pixxio-cli/internal/ui"
)

// DirectoriesCmd is the directories command group.
type DirectoriesCmd struct {
	List DirectoriesListCmd `cmd:"" help:"List directories."`
}

type DirectoriesListCmd struct {
	PageSize int    `help:"Number of directories per page." short:"l" default:"20"`
	Page     int    `help:"Page number." default:"1"`
	Name     string `help:"Filter by name (wildcard)." short:"s"`
}

func (c *DirectoriesListCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if err := requireAuth(cfg); err != nil {
		return err
	}

	client := newAPIClient(cfg.Mediaspace, cfg.APIKey)
	result, err := client.ListDirectories(ctx, pixxapi.ListDirectoriesOptions{
		PageSize: c.PageSize,
		Page:     c.Page,
		Name:     c.Name,
	})
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, result)
	}
	if outfmt.IsPlain(ctx) {
		for _, d := range result.Directories {
			fmt.Fprintf(os.Stdout, "%d\t%s\t%s\t%d\n", d.ID, d.Name, d.Path, d.Quantity)
		}
		return nil
	}

	u.Out().Printf("Directories (%d total):", result.Quantity)
	for _, d := range result.Directories {
		path := d.Path
		if path == "" {
			path = d.Name
		}
		u.Out().Printf("  %d  %s  (%d files)", d.ID, path, d.Quantity)
	}
	return nil
}
