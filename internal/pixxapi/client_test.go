package pixxapi

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: create a client pointing at an httptest server.
func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New("test.px.media", "test-key")
	c.Mediaspace = srv.Listener.Addr().String()
	// Override baseURL to use http:// and the test server address.
	c.HTTPClient = srv.Client()
	// Patch: we need to override how baseURL works for tests.
	// Easiest: set Mediaspace to the full host so baseURL returns http://host.
	c.Mediaspace = strings.TrimPrefix(srv.URL, "http://")
	// But baseURL prepends https://, so we need a custom transport.
	// Simpler: just override HTTPClient to redirect to the test server.
	c.HTTPClient = &http.Client{
		Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport},
	}
	return c
}

// rewriteTransport rewrites all requests to point at the test server.
type rewriteTransport struct {
	base    string
	wrapped http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.base, "http://")
	return t.wrapped.RoundTrip(req)
}

func TestListFiles(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/files") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth header = %q", got)
		}
		if r.URL.Query().Get("pageSize") != "10" {
			t.Errorf("pageSize = %q", r.URL.Query().Get("pageSize"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"quantity":1,"files":[{"id":42,"fileName":"cat.jpg","fileSize":1234}]}`)
	}

	c := testClient(t, handler)
	result, err := c.ListFiles(context.Background(), ListFilesOptions{PageSize: 10, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Quantity != 1 {
		t.Errorf("quantity = %d", result.Quantity)
	}
	if result.Files[0].ID != 42 {
		t.Errorf("file ID = %d", result.Files[0].ID)
	}
	if result.Files[0].FileName != "cat.jpg" {
		t.Errorf("fileName = %q", result.Files[0].FileName)
	}
}

func TestListFiles_Unauthorized(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"success":false,"errorMessage":"invalid token"}`)
	}
	c := testClient(t, handler)
	_, err := c.ListFiles(context.Background(), ListFilesOptions{PageSize: 5})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q", err)
	}
}

func TestListFiles_NoAPIKey(t *testing.T) {
	c := New("test.px.media", "")
	_, err := c.ListFiles(context.Background(), ListFilesOptions{})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("error = %q", err)
	}
}

func TestDownloadFile(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/files/99/convert") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("downloadType") != "original" {
			t.Errorf("downloadType = %q", r.URL.Query().Get("downloadType"))
		}
		if r.URL.Query().Get("responseType") != "path" {
			t.Errorf("responseType = %q", r.URL.Query().Get("responseType"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"downloadURL":"https://cdn.example.com/file.jpg","fileSize":5678}`)
	}
	c := testClient(t, handler)
	result, err := c.DownloadFile(context.Background(), 99, "original")
	if err != nil {
		t.Fatal(err)
	}
	if result.DownloadURL != "https://cdn.example.com/file.jpg" {
		t.Errorf("downloadURL = %q", result.DownloadURL)
	}
	if result.FileSize != 5678 {
		t.Errorf("fileSize = %d", result.FileSize)
	}
}

func TestUploadFile(t *testing.T) {
	// Create a temp file to upload.
	tmp := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmp, []byte("hello pixx"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/files" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil || mt != "multipart/form-data" {
			t.Errorf("content-type = %q", ct)
		}
		// Verify the file field exists.
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		f, fh, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if fh.Filename != "test.txt" {
			t.Errorf("filename = %q", fh.Filename)
		}
		body, _ := io.ReadAll(f)
		if string(body) != "hello pixx" {
			t.Errorf("body = %q", body)
		}
		if r.FormValue("subject") != "Test Subject" {
			t.Errorf("subject = %q", r.FormValue("subject"))
		}
		if r.FormValue("asynchronousConversion") != "false" {
			t.Errorf("async = %q", r.FormValue("asynchronousConversion"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"id":77,"isDuplicate":false}`)
	}
	c := testClient(t, handler)
	result, err := c.UploadFile(context.Background(), tmp, UploadOptions{Subject: "Test Subject"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != 77 {
		t.Errorf("id = %d", result.ID)
	}
	if result.IsDuplicate {
		t.Error("unexpected duplicate")
	}
}

func TestListCollections(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/collections" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"quantity":2,"collections":[{"id":1,"name":"Brand Assets","filesQuantity":10,"isDynamic":false},{"id":2,"name":"Recent","filesQuantity":5,"isDynamic":true}]}`)
	}
	c := testClient(t, handler)
	result, err := c.ListCollections(context.Background(), ListCollectionsOptions{PageSize: 20, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Quantity != 2 {
		t.Errorf("quantity = %d", result.Quantity)
	}
	if result.Collections[0].Name != "Brand Assets" {
		t.Errorf("name = %q", result.Collections[0].Name)
	}
	if result.Collections[1].IsDynamic != true {
		t.Error("expected dynamic")
	}
}

func TestGetFile(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/files/42") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"file":{"id":42,"fileName":"logo.png","fileSize":9999,"fileType":"image","width":800,"height":600,"keywords":["brand","logo"]}}`)
	}
	c := testClient(t, handler)
	result, err := c.GetFile(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if result.File.ID != 42 {
		t.Errorf("id = %d", result.File.ID)
	}
	if result.File.Width != 800 {
		t.Errorf("width = %d", result.File.Width)
	}
	if len(result.File.Keywords) != 2 || result.File.Keywords[0] != "brand" {
		t.Errorf("keywords = %v", result.File.Keywords)
	}
}

func TestDeleteFiles(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/files" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ids := r.URL.Query().Get("ids")
		if ids != "[1,2,3]" {
			t.Errorf("ids = %q", ids)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"jobID":555}`)
	}
	c := testClient(t, handler)
	result, err := c.DeleteFiles(context.Background(), []int{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if result.JobID != 555 {
		t.Errorf("jobID = %d", result.JobID)
	}
}

func TestListDirectories(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/directories" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "Photos" {
			t.Errorf("name filter = %q", r.URL.Query().Get("name"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"quantity":1,"directories":[{"id":5,"name":"Photos","path":"/Photos","quantity":42}]}`)
	}
	c := testClient(t, handler)
	result, err := c.ListDirectories(context.Background(), ListDirectoriesOptions{PageSize: 20, Page: 1, Name: "Photos"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Quantity != 1 {
		t.Errorf("quantity = %d", result.Quantity)
	}
	if result.Directories[0].Path != "/Photos" {
		t.Errorf("path = %q", result.Directories[0].Path)
	}
	if result.Directories[0].Quantity != 42 {
		t.Errorf("file count = %d", result.Directories[0].Quantity)
	}
}
