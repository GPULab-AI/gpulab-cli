package api

import (
	"bytes"
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

// UploadVolumeFiles uploads one or more local files into destDir on the volume.
func (c *Client) UploadVolumeFiles(volumeUUID, destDir string, localPaths []string) ([]byte, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("path", destDir); err != nil {
		return nil, err
	}

	for _, localPath := range localPaths {
		file, err := os.Open(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", localPath, err)
		}

		part, err := writer.CreateFormFile("files[]", filepath.Base(localPath))
		if err != nil {
			file.Close()
			return nil, err
		}
		if _, err := io.Copy(part, file); err != nil {
			file.Close()
			return nil, err
		}
		file.Close()

		// Flat upload: keep files at the destination root.
		if err := writer.WriteField("relative_paths[]", ""); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return c.doRaw("POST", c.volumeFilesPath(volumeUUID, "/upload"), writer.FormDataContentType(), &buf)
}
