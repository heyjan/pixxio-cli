package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/heyjan/pixxio-cli/internal/config"
	"github.com/heyjan/pixxio-cli/internal/outfmt"
	"github.com/heyjan/pixxio-cli/internal/pixxapi"
	"github.com/heyjan/pixxio-cli/internal/ui"
)

// newAPIClient is a package-level function variable for testability.
var newAPIClient = func(mediaspace, apiKey string) *pixxapi.Client {
	return pixxapi.New(mediaspace, apiKey)
}

// FilesCmd is the files command group.
type FilesCmd struct {
	List     FilesListCmd     `cmd:"" help:"List files in the mediaspace."`
	Get      FilesGetCmd      `cmd:"" help:"Get details for a file."`
	Download FilesDownloadCmd `cmd:"" help:"Download a file by ID."`
	Upload   FilesUploadCmd   `cmd:"" help:"Upload a file."`
	Delete   FilesDeleteCmd   `cmd:"" help:"Delete files by ID."`
}

func requireAuth(cfg *config.Config) error {
	if cfg.Mediaspace == "" || cfg.APIKey == "" {
		return usage("not authenticated — run: pixx auth login")
	}
	return nil
}

// --- list ---

type FilesListCmd struct {
	PageSize int `help:"Number of files per page." short:"l" default:"20"`
	Page     int `help:"Page number." default:"1"`
}

func (c *FilesListCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if err := requireAuth(cfg); err != nil {
		return err
	}

	client := newAPIClient(cfg.Mediaspace, cfg.APIKey)
	result, err := client.ListFiles(ctx, pixxapi.ListFilesOptions{
		PageSize: c.PageSize, Page: c.Page,
	})
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, result)
	}
	if outfmt.IsPlain(ctx) {
		for _, f := range result.Files {
			fmt.Fprintf(os.Stdout, "%d\t%s\t%d\n", f.ID, f.FileName, f.FileSize)
		}
		return nil
	}
	u.Out().Printf("Files (%d total):", result.Quantity)
	for _, f := range result.Files {
		u.Out().Printf("  %d  %s  (%d bytes)", f.ID, f.FileName, f.FileSize)
	}
	return nil
}

// --- download ---

type FilesDownloadCmd struct {
	ID   int    `arg:"" help:"File ID to download."`
	Out  string `help:"Output path (default: original filename in cwd)." short:"o"`
	Type string `help:"Download type: original|preview." default:"original" enum:"original,preview"`
}

func (c *FilesDownloadCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if err := requireAuth(cfg); err != nil {
		return err
	}

	client := newAPIClient(cfg.Mediaspace, cfg.APIKey)

	// Get download URL.
	dl, err := client.DownloadFile(ctx, c.ID, c.Type)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, dl)
	}

	// Fetch the actual file bytes from the download URL.
	resp, err := http.Get(dl.DownloadURL) //nolint:gosec
	if err != nil {
		return fmt.Errorf("fetch file: %w", err)
	}
	defer resp.Body.Close()

	outPath := c.Out
	if outPath == "" {
		// Derive filename from Content-Disposition or URL.
		outPath = filepath.Base(dl.DownloadURL)
		if outPath == "" || outPath == "." {
			outPath = fmt.Sprintf("file-%d", c.ID)
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	u.Out().Successf("Downloaded %s (%d bytes)", outPath, n)
	return nil
}

// --- upload ---

type FilesUploadCmd struct {
	File        string `arg:"" help:"Path to file to upload." type:"existingfile"`
	DirectoryID int    `help:"Target directory ID." short:"d"`
	Subject     string `help:"Subject/title for the file." short:"s"`
	Description string `help:"Description for the file."`
}

func (c *FilesUploadCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if err := requireAuth(cfg); err != nil {
		return err
	}

	client := newAPIClient(cfg.Mediaspace, cfg.APIKey)
	result, err := client.UploadFile(ctx, c.File, pixxapi.UploadOptions{
		DirectoryID: c.DirectoryID,
		Subject:     c.Subject,
		Description: c.Description,
	})
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, result)
	}

	if result.IsDuplicate {
		u.Out().Warnf("Duplicate detected — file ID %d", result.ID)
	} else {
		u.Out().Successf("Uploaded %s → file ID %d", filepath.Base(c.File), result.ID)
	}
	return nil
}

// --- get ---

type FilesGetCmd struct {
	ID int `arg:"" help:"File ID."`
}

func (c *FilesGetCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if err := requireAuth(cfg); err != nil {
		return err
	}

	client := newAPIClient(cfg.Mediaspace, cfg.APIKey)
	result, err := client.GetFile(ctx, c.ID)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, result)
	}

	f := result.File
	if outfmt.IsPlain(ctx) {
		fmt.Fprintf(os.Stdout, "%d\t%s\t%d\t%s\t%s\n", f.ID, f.FileName, f.FileSize, f.FileType, f.Subject)
		return nil
	}

	u.Out().Printf("File %d", f.ID)
	u.Out().Printf("  Name:     %s", f.FileName)
	u.Out().Printf("  Size:     %d bytes", f.FileSize)
	if f.FileType != "" {
		u.Out().Printf("  Type:     %s", f.FileType)
	}
	if f.Width > 0 {
		u.Out().Printf("  Dims:     %dx%d", f.Width, f.Height)
	}
	if f.Subject != "" {
		u.Out().Printf("  Subject:  %s", f.Subject)
	}
	if f.Description != "" {
		u.Out().Printf("  Desc:     %s", f.Description)
	}
	if len(f.Keywords) > 0 {
		u.Out().Printf("  Keywords: %s", strings.Join(f.Keywords, ", "))
	}
	if f.UploadDate != "" {
		u.Out().Printf("  Uploaded: %s", f.UploadDate)
	}
	return nil
}

// --- delete ---

type FilesDeleteCmd struct {
	IDs []int `arg:"" help:"File IDs to delete." required:""`
}

func (c *FilesDeleteCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if err := requireAuth(cfg); err != nil {
		return err
	}

	if flags.DryRun {
		u.Err().Warnf("[dry-run] Would delete %d file(s): %v", len(c.IDs), c.IDs)
		return nil
	}

	if !flags.Force {
		u.Err().Warnf("About to delete %d file(s): %v", len(c.IDs), c.IDs)
		u.Err().Printf("Use --force / -y to skip this confirmation, or --dry-run to preview.")
		return usage("confirmation required — pass --force to delete")
	}

	client := newAPIClient(cfg.Mediaspace, cfg.APIKey)
	result, err := client.DeleteFiles(ctx, c.IDs)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, result)
	}

	u.Out().Successf("Deleted %d file(s) (job %d)", len(c.IDs), result.JobID)
	return nil
}
