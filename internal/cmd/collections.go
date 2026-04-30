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

// CollectionsCmd is the collections command group.
type CollectionsCmd struct {
	List CollectionsListCmd `cmd:"" help:"List collections."`
}

// CollectionsListCmd lists collections.
type CollectionsListCmd struct {
	PageSize int `help:"Number of collections per page." short:"l" default:"20"`
	Page     int `help:"Page number." default:"1"`
}

func (c *CollectionsListCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if cfg.Mediaspace == "" || cfg.APIKey == "" {
		return usage("not authenticated — run: pixx auth login")
	}

	client := newAPIClient(cfg.Mediaspace, cfg.APIKey)
	result, err := client.ListCollections(ctx, pixxapi.ListCollectionsOptions{
		PageSize: c.PageSize,
		Page:     c.Page,
	})
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, result)
	}
	if outfmt.IsPlain(ctx) {
		for _, col := range result.Collections {
			fmt.Fprintf(os.Stdout, "%d\t%s\t%d\t%v\n", col.ID, col.Name, col.FilesQuantity, col.IsDynamic)
		}
		return nil
	}

	u.Out().Printf("Collections (%d total):", result.Quantity)
	for _, col := range result.Collections {
		kind := "static"
		if col.IsDynamic {
			kind = "dynamic"
		}
		u.Out().Printf("  %d  %s  (%d files, %s)", col.ID, col.Name, col.FilesQuantity, kind)
	}
	return nil
}
