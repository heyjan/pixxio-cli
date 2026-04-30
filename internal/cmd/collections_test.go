package cmd

import (
	"testing"

	"github.com/heyjan/pixxio-cli/internal/config"
	"github.com/heyjan/pixxio-cli/internal/outfmt"
)

func TestCollectionsListCmd_NoAuth(t *testing.T) {
	ctx, _ := testContext(t, &config.Config{}, outfmt.ModeInteractive)
	cmd := &CollectionsListCmd{PageSize: 10, Page: 1}
	err := cmd.Run(ctx, &RootFlags{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectionsListCmd_AuthRequired(t *testing.T) {
	ctx, _ := testContext(t, &config.Config{Mediaspace: "demo"}, outfmt.ModeInteractive)
	cmd := &CollectionsListCmd{PageSize: 10, Page: 1}
	err := cmd.Run(ctx, &RootFlags{})
	if err == nil {
		t.Fatal("expected auth error when APIKey missing")
	}
}
