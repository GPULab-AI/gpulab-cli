package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
)

// localFileDigest streams a local file and returns its sha256 hex digest and size
// without buffering it in memory.
func localFileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// remoteDiffersFromLocal decides whether localPath needs to be uploaded to
// remotePath given the already-known remote size. It short-circuits on size
// (different size ⇒ changed) and only content-hashes when the sizes match — the
// exact case a size-only diff gets wrong (an equal-length edit). With sizeOnly
// it trusts size alone.
//
// remoteExists=false means the file isn't on the volume yet (always "new").
func remoteDiffersFromLocal(client *api.Client, uuid, remotePath, localPath string, localSize, remoteSize int64, remoteExists, sizeOnly bool) (changed bool, reason string, err error) {
	if !remoteExists {
		return true, "new", nil
	}
	if remoteSize != localSize {
		return true, "size", nil
	}
	if sizeOnly {
		return false, "", nil
	}
	// Same size — the dangerous case. Content-hash both sides.
	localSum, _, err := localFileDigest(localPath)
	if err != nil {
		return false, "", err
	}
	remoteSum, _, found, err := client.HashVolumeFile(uuid, remotePath)
	if err != nil {
		return false, "", fmt.Errorf("hashing remote %s: %w", remotePath, err)
	}
	if !found {
		return true, "new", nil
	}
	if remoteSum != localSum {
		return true, "content", nil
	}
	return false, "", nil
}

// verifyReadback confirms the file uploaded to remotePath is byte-identical to
// localPath by comparing sha256 digests, streaming both sides (no full-file
// buffering, so it is safe for multi-GB files).
func verifyReadback(client *api.Client, uuid, remotePath, localPath string) error {
	localSum, _, err := localFileDigest(localPath)
	if err != nil {
		return err
	}
	remoteSum, _, found, err := client.HashVolumeFile(uuid, remotePath)
	if err != nil {
		return fmt.Errorf("verify: cannot read back %s: %w", remotePath, err)
	}
	if !found {
		return fmt.Errorf("verify: %s is missing after upload", remotePath)
	}
	if remoteSum != localSum {
		return fmt.Errorf("verify FAILED for %s: readback differs from local (upload corrupted)", remotePath)
	}
	return nil
}
