package pixxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/heyjan/pixxio-cli/internal/errfmt"
)

// Client talks to the pixx.io API using Bearer auth.
type Client struct {
	Mediaspace string
	APIKey     string
	HTTPClient *http.Client
}

func New(mediaspace, apiKey string) *Client {
	return &Client{
		Mediaspace: mediaspace,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) baseURL() string {
	ms := c.Mediaspace
	if !strings.Contains(ms, ".") {
		ms += ".px.media"
	}
	return "https://" + ms
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	return req, nil
}

func (c *Client) checkAuth() error {
	if c.APIKey == "" {
		return &errfmt.AuthRequiredError{Mediaspace: c.Mediaspace}
	}
	return nil
}

func apiError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return &errfmt.APIError{StatusCode: resp.StatusCode, Message: string(body)}
}

// --- Files ---

type ListFilesOptions struct {
	PageSize int
	Page     int
}

type FileListResponse struct {
	Success  bool   `json:"success"`
	Files    []File `json:"files"`
	Quantity int    `json:"quantity"`
}

type File struct {
	ID         int    `json:"id"`
	FileName   string `json:"fileName"`
	FileSize   int64  `json:"fileSize"`
	Subject    string `json:"subject,omitempty"`
	UploadDate string `json:"uploadDate,omitempty"`
}

func (c *Client) ListFiles(ctx context.Context, opts ListFilesOptions) (*FileListResponse, error) {
	if err := c.checkAuth(); err != nil {
		return nil, err
	}
	params := url.Values{}
	if opts.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	if opts.Page > 0 {
		params.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	params.Set("responseFields", `["id","fileName","fileSize","subject","uploadDate"]`)

	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/files?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var result FileListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode files: %w", err)
	}
	return &result, nil
}

// DownloadFile downloads a file by ID, returning a temporary download URL.
// GET /api/v1/files/{id}/convert?downloadType=original&responseType=path
func (c *Client) DownloadFile(ctx context.Context, fileID int, downloadType string) (*DownloadResponse, error) {
	if err := c.checkAuth(); err != nil {
		return nil, err
	}
	if downloadType == "" {
		downloadType = "original"
	}
	params := url.Values{
		"downloadType": {downloadType},
		"responseType": {"path"},
	}
	path := fmt.Sprintf("/api/v1/files/%d/convert?%s", fileID, params.Encode())
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var result DownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode download: %w", err)
	}
	return &result, nil
}

type DownloadResponse struct {
	Success     bool   `json:"success"`
	DownloadURL string `json:"downloadURL"`
	FileSize    int64  `json:"fileSize,omitempty"`
}

// UploadFile uploads a local file. POST /api/v1/files (multipart/form-data)
func (c *Client) UploadFile(ctx context.Context, filePath string, opts UploadOptions) (*UploadResponse, error) {
	if err := c.checkAuth(); err != nil {
		return nil, err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fw, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}

	if opts.DirectoryID > 0 {
		_ = w.WriteField("directoryID", fmt.Sprintf("%d", opts.DirectoryID))
	}
	if opts.Subject != "" {
		_ = w.WriteField("subject", opts.Subject)
	}
	if opts.Description != "" {
		_ = w.WriteField("description", opts.Description)
	}
	// Synchronous so we get the file ID back immediately.
	_ = w.WriteField("asynchronousConversion", "false")
	w.Close()

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/files", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var result UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode upload: %w", err)
	}
	return &result, nil
}

type UploadOptions struct {
	DirectoryID int
	Subject     string
	Description string
}

type UploadResponse struct {
	Success     bool `json:"success"`
	ID          int  `json:"id,omitempty"`
	JobID       int  `json:"jobID,omitempty"`
	IsDuplicate bool `json:"isDuplicate,omitempty"`
}

// --- Collections ---

type CollectionListResponse struct {
	Success     bool         `json:"success"`
	Collections []Collection `json:"collections"`
	Quantity    int          `json:"quantity"`
}

type Collection struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	FilesQuantity int    `json:"filesQuantity"`
	IsDynamic     bool   `json:"isDynamic"`
	CreateDate    string `json:"createDate,omitempty"`
}

type ListCollectionsOptions struct {
	PageSize int
	Page     int
}

func (c *Client) ListCollections(ctx context.Context, opts ListCollectionsOptions) (*CollectionListResponse, error) {
	if err := c.checkAuth(); err != nil {
		return nil, err
	}
	params := url.Values{}
	if opts.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	if opts.Page > 0 {
		params.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	params.Set("responseFields", `["id","name","description","filesQuantity","isDynamic","createDate"]`)

	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/collections?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var result CollectionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode collections: %w", err)
	}
	return &result, nil
}

// --- File Detail ---

// FileDetail is the full file object from GET /api/v1/files/{id}.
type FileDetail struct {
	ID          int      `json:"id"`
	FileName    string   `json:"fileName"`
	FileSize    int64    `json:"fileSize"`
	Subject     string   `json:"subject,omitempty"`
	Description string   `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	UploadDate  string   `json:"uploadDate,omitempty"`
	CreateDate  string   `json:"createDate,omitempty"`
	ModifyDate  string   `json:"modifyDate,omitempty"`
	FileType    string   `json:"fileType,omitempty"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
	PreviewURL  string   `json:"previewFileURL,omitempty"`
}

type FileDetailResponse struct {
	Success bool       `json:"success"`
	File    FileDetail `json:"file"`
}

// GetFile retrieves details for a single file. GET /api/v1/files/{id}
func (c *Client) GetFile(ctx context.Context, fileID int) (*FileDetailResponse, error) {
	if err := c.checkAuth(); err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("responseFields", `["id","fileName","fileSize","subject","description","keywords","uploadDate","createDate","modifyDate","fileType","width","height","previewFileURL"]`)
	path := fmt.Sprintf("/api/v1/files/%d?%s", fileID, params.Encode())
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var result FileDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode file detail: %w", err)
	}
	return &result, nil
}

// --- File Delete ---

type DeleteFilesResponse struct {
	Success bool `json:"success"`
	JobID   int  `json:"jobID,omitempty"`
}

// DeleteFiles deletes files by IDs. DELETE /api/v1/files?ids=[...]
func (c *Client) DeleteFiles(ctx context.Context, ids []int) (*DeleteFilesResponse, error) {
	if err := c.checkAuth(); err != nil {
		return nil, err
	}
	idsJSON, _ := json.Marshal(ids)
	params := url.Values{"ids": {string(idsJSON)}}
	req, err := c.newRequest(ctx, http.MethodDelete, "/api/v1/files?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var result DeleteFilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode delete: %w", err)
	}
	return &result, nil
}

// --- Directories ---

type Directory struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID int    `json:"parentID,omitempty"`
	Path     string `json:"path,omitempty"`
	Quantity int    `json:"quantity"`
}

type DirectoryListResponse struct {
	Success     bool        `json:"success"`
	Directories []Directory `json:"directories"`
	Quantity    int         `json:"quantity"`
}

type ListDirectoriesOptions struct {
	PageSize int
	Page     int
	Name     string
}

// ListDirectories retrieves directories. GET /api/v1/directories
func (c *Client) ListDirectories(ctx context.Context, opts ListDirectoriesOptions) (*DirectoryListResponse, error) {
	if err := c.checkAuth(); err != nil {
		return nil, err
	}
	params := url.Values{}
	if opts.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	if opts.Page > 0 {
		params.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if opts.Name != "" {
		params.Set("name", opts.Name)
	}
	params.Set("responseFields", `["id","name","parentID","path","quantity"]`)
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/directories?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var result DirectoryListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode directories: %w", err)
	}
	return &result, nil
}
