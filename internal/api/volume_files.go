package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
)

// VolumeFile is a single entry returned by the volume file listing/search.
type VolumeFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDirectory bool   `json:"is_directory"`
	IsText      bool   `json:"is_text"`
	Size        int64  `json:"size"`
	Modified    string `json:"modified"`
	Extension   string `json:"extension"`
}

// VolumeDirInfo summarizes a directory listing.
type VolumeDirInfo struct {
	Directories int `json:"directories"`
	Files       int `json:"files"`
}

// VolumeFileListResponse is the envelope returned by list and search.
type VolumeFileListResponse struct {
	Status  string         `json:"status"`
	Items   []VolumeFile   `json:"items"`
	Info    *VolumeDirInfo `json:"info"`
	Message string         `json:"message"`
}

func (c *Client) volumeFilesPath(volumeUUID, suffix string) string {
	return "/v1/volumes/" + url.PathEscape(volumeUUID) + "/files" + suffix
}

// ListVolumeFiles lists files in a directory within a volume.
func (c *Client) ListVolumeFiles(volumeUUID, path string) (*VolumeFileListResponse, error) {
	endpoint := c.volumeFilesPath(volumeUUID, "")
	if path != "" {
		endpoint += "?path=" + url.QueryEscape(path)
	}
	data, err := c.Get(endpoint)
	if err != nil {
		return nil, err
	}
	var resp VolumeFileListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse file list: %w", err)
	}
	return &resp, nil
}

// SearchVolumeFiles searches for files within a volume.
func (c *Client) SearchVolumeFiles(volumeUUID, query, path string) (*VolumeFileListResponse, error) {
	q := url.Values{}
	q.Set("query", query)
	if path != "" {
		q.Set("path", path)
	}
	data, err := c.Get(c.volumeFilesPath(volumeUUID, "/search") + "?" + q.Encode())
	if err != nil {
		return nil, err
	}
	var resp VolumeFileListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}
	return &resp, nil
}

// GetVolumeFileContent returns the text content of a file for viewing/editing.
func (c *Client) GetVolumeFileContent(volumeUUID, path string) (string, error) {
	data, err := c.Get(c.volumeFilesPath(volumeUUID, "/content") + "?path=" + url.QueryEscape(path))
	if err != nil {
		return "", err
	}
	var resp struct {
		Status  string `json:"status"`
		Content string `json:"content"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("failed to parse file content: %w", err)
	}
	return resp.Content, nil
}

// SaveVolumeFileContent overwrites (or creates) a file with the given content.
func (c *Client) SaveVolumeFileContent(volumeUUID, path, content string) error {
	body := map[string]string{"path": path, "content": content}
	_, err := c.Put(c.volumeFilesPath(volumeUUID, "/content"), body)
	return err
}

// CreateVolumeFile creates a new file with optional content.
func (c *Client) CreateVolumeFile(volumeUUID, path, content string) error {
	body := map[string]string{"path": path, "content": content}
	_, err := c.Post(c.volumeFilesPath(volumeUUID, "/create-file"), body)
	return err
}

// CreateVolumeDirectory creates a directory.
func (c *Client) CreateVolumeDirectory(volumeUUID, path string) error {
	_, err := c.Post(c.volumeFilesPath(volumeUUID, "/mkdir"), map[string]string{"path": path})
	return err
}

// DeleteVolumePath deletes a file or directory.
func (c *Client) DeleteVolumePath(volumeUUID, path string) error {
	_, err := c.Delete(c.volumeFilesPath(volumeUUID, "") + "?path=" + url.QueryEscape(path))
	return err
}

// RenameVolumePath renames/moves a file or directory.
func (c *Client) RenameVolumePath(volumeUUID, oldPath, newPath string) error {
	body := map[string]string{"old_path": oldPath, "new_path": newPath}
	_, err := c.Post(c.volumeFilesPath(volumeUUID, "/rename"), body)
	return err
}

// CopyVolumePath copies a file or directory.
func (c *Client) CopyVolumePath(volumeUUID, source, dest string) error {
	body := map[string]string{"source_path": source, "dest_path": dest}
	_, err := c.Post(c.volumeFilesPath(volumeUUID, "/copy"), body)
	return err
}

// DownloadVolumeFile returns the raw bytes of a file.
func (c *Client) DownloadVolumeFile(volumeUUID, path string) ([]byte, error) {
	return c.Get(c.volumeFilesPath(volumeUUID, "/download") + "?path=" + url.QueryEscape(path))
}

// UploadItem is a single file to upload. DestName overrides the stored filename
// (default: the local basename), letting callers rename on upload.
type UploadItem struct {
	LocalPath string
	DestName  string
}

// UploadVolumeFiles uploads local files into destDir on the volume. Each file is
// stored flat under destDir using its DestName (or basename). To place a file at
// an arbitrary path, set destDir to the target directory and DestName to the
// target filename.
//
// The multipart body is streamed through an io.Pipe so multi-GB files are never
// buffered fully in memory.
func (c *Client) UploadVolumeFiles(volumeUUID, destDir string, items []UploadItem) ([]byte, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		// Any error here is surfaced on the read side via CloseWithError, which
		// makes the HTTP request fail with the same error.
		err := func() error {
			if err := writer.WriteField("path", destDir); err != nil {
				return err
			}
			for _, item := range items {
				file, err := os.Open(item.LocalPath)
				if err != nil {
					return fmt.Errorf("failed to open %s: %w", item.LocalPath, err)
				}
				name := item.DestName
				if name == "" {
					name = filepath.Base(item.LocalPath)
				}
				part, err := writer.CreateFormFile("files[]", name)
				if err != nil {
					file.Close()
					return err
				}
				if _, err := io.Copy(part, file); err != nil {
					file.Close()
					return err
				}
				file.Close()
				// Flat upload: system-server stores each by its multipart filename.
				if err := writer.WriteField("relative_paths[]", ""); err != nil {
					return err
				}
			}
			return writer.Close()
		}()
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	return c.PostStream(c.volumeFilesPath(volumeUUID, "/upload"), writer.FormDataContentType(), pr)
}

// HashVolumeFile streams a file from the volume and returns its sha256 and size
// without buffering it in memory. found=false (nil error) for a missing file.
func (c *Client) HashVolumeFile(volumeUUID, path string) (sum string, size int64, found bool, err error) {
	resp, err := c.GetStream(c.volumeFilesPath(volumeUUID, "/download") + "?path=" + url.QueryEscape(path))
	if err != nil {
		if IsNotFound(err) {
			return "", 0, false, nil
		}
		return "", 0, false, err
	}
	defer resp.Body.Close()

	h := sha256.New()
	n, err := io.Copy(h, resp.Body)
	if err != nil {
		return "", 0, false, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, true, nil
}

// UploadLocalFiles is a convenience wrapper for uploading files by path using
// their basenames.
func (c *Client) UploadLocalFiles(volumeUUID, destDir string, localPaths []string) ([]byte, error) {
	items := make([]UploadItem, len(localPaths))
	for i, p := range localPaths {
		items[i] = UploadItem{LocalPath: p}
	}
	return c.UploadVolumeFiles(volumeUUID, destDir, items)
}
