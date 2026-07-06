package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	syncDelete     bool
	syncYes        bool
	syncDryRun     bool
	syncDepth      int
	syncIncludeAll bool
	syncSizeOnly   bool
	syncNoVerify   bool
	syncAllowEmpty bool
)

func init() {
	volumesCmd.AddCommand(volumesSyncCmd)
	volumesSyncCmd.Flags().BoolVar(&syncDelete, "delete", false, "Delete remote files that don't exist locally")
	volumesSyncCmd.Flags().BoolVarP(&syncYes, "yes", "y", false, "Skip confirmation")
	volumesSyncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show the plan and exit without changing anything")
	volumesSyncCmd.Flags().IntVar(&syncDepth, "depth", 20, "Max remote directory depth to scan")
	volumesSyncCmd.Flags().BoolVar(&syncIncludeAll, "all", false, "Include hidden/secret files (.git, __pycache__, *.pyc, .env, keys)")
	volumesSyncCmd.Flags().BoolVar(&syncSizeOnly, "size-only", false, "Compare by size only (faster, but misses equal-length edits)")
	volumesSyncCmd.Flags().BoolVar(&syncNoVerify, "no-verify", false, "Skip hash verification of uploaded files")
	volumesSyncCmd.Flags().BoolVar(&syncAllowEmpty, "allow-empty", false, "Allow --delete when the local dir has no files (dangerous)")
}

type syncAction struct {
	Rel      string
	Local    string
	Remote   string
	Kind     string // new | changed | delete
	LocalSz  int64
	RemoteSz int64
}

var volumesSyncCmd = &cobra.Command{
	Use:   "sync [LOCAL_DIR] [VOLUME] [REMOTE_DIR]",
	Short: "Sync a local directory to a volume (rsync-style, with a diff preview)",
	Long: `Upload a local directory to a network volume, uploading only files that are new
or changed. Files are compared by content hash (so an equal-length edit is still
detected); pass --size-only for a faster size-only comparison. Uploaded files
are read back and hash-verified unless --no-verify. Prints a diff preview and
how many serverless endpoints/replicas use the volume before doing anything —
those replicas keep running the old code until they are restarted (see
'serverless deploy').

REMOTE_DIR scopes the sync to a subdirectory of the volume (default: root).

Examples:
  gpulab volumes sync ./server my-vol server           # preview + upload changes
  gpulab volumes sync ./server my-vol server --dry-run  # preview only
  gpulab volumes sync ./server my-vol server --delete   # also remove extra remote files`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth().WithTimeout(30 * time.Minute)

		localDir := args[0]
		info, err := os.Stat(localDir)
		if err != nil {
			return fmt.Errorf("cannot read local dir %s: %w", localDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory (use 'volumes files upload' for a single file)", localDir)
		}

		uuid, err := client.ResolveVolumeUUID(args[1])
		if err != nil {
			return err
		}
		remoteRoot := ""
		if len(args) == 3 {
			remoteRoot = strings.Trim(args[2], "/")
		}

		localFiles, err := walkLocalDir(localDir, syncIncludeAll)
		if err != nil {
			return err
		}

		// Refuse to delete everything from an empty/wrong local dir unless the
		// caller explicitly opts in — this is the volume-wipe footgun.
		if syncDelete && len(localFiles) == 0 && !syncAllowEmpty {
			return fmt.Errorf("refusing to --delete with an empty local dir %q (this would wipe the volume); pass --allow-empty to override", localDir)
		}

		// Only a genuine 404 means "remote dir doesn't exist yet" (first sync);
		// a 403/500/wrong-volume must abort, not masquerade as an empty remote.
		remoteFiles := map[string]int64{}
		remoteList, truncated, rerr := listVolumeFilesRecursive(client, uuid, remoteRoot, syncDepth)
		if rerr != nil && !api.IsNotFound(rerr) {
			return fmt.Errorf("listing remote %q failed: %w", remoteRoot, rerr)
		}
		prefix := ""
		if remoteRoot != "" {
			prefix = remoteRoot + "/"
		}
		for _, f := range remoteList {
			remoteFiles[strings.TrimPrefix(f.Path, prefix)] = f.Size
		}
		if truncated && !flagJSON {
			fmt.Fprintf(os.Stderr, "warning: remote scan hit the depth limit (%d); files deeper than that are ignored — raise --depth\n", syncDepth)
		}

		actions, err := diffSync(client, uuid, localDir, remoteRoot, localFiles, remoteFiles, syncDelete, syncSizeOnly)
		if err != nil {
			return err
		}
		sort.Slice(actions, func(i, j int) bool { return actions[i].Rel < actions[j].Rel })

		// Who is affected — a strong signal that replicas won't pick up the
		// change until restarted.
		services, _ := client.ListServerlessServices()
		affected := filterServicesByVolume(services, uuid)
		replicaCount := 0
		for _, s := range affected {
			replicaCount += s.ActiveReplicasCount
		}

		unchanged := len(localFiles) - countKind(actions, "new") - countKind(actions, "changed")
		pending := countKind(actions, "new") + countKind(actions, "changed") + countKind(actions, "delete")

		// Whether we will actually mutate the volume. Non-interactive callers
		// (--json, no TTY) must pass --yes; we never silently apply a plan they
		// couldn't confirm — especially --delete.
		willApply := pending > 0 && !syncDryRun && (syncYes || !flagJSON)

		if flagJSON {
			output.PrintJSON(map[string]interface{}{
				"volume":             uuid,
				"remote_dir":         remoteRoot,
				"new":                countKind(actions, "new"),
				"changed":            countKind(actions, "changed"),
				"deleted":            countKind(actions, "delete"),
				"unchanged":          unchanged,
				"affected_endpoints": len(affected),
				"affected_replicas":  replicaCount,
				"actions":            actions,
				"verified":           !syncNoVerify,
				"applied":            willApply && syncYes,
			})
		} else {
			printSyncPlan(actions, unchanged, len(affected), replicaCount, uuid, remoteRoot)
		}

		if pending == 0 {
			if !flagJSON {
				fmt.Println("Already in sync.")
			}
			return nil
		}
		if syncDryRun {
			return nil
		}
		if flagJSON && !syncYes {
			// Preview only — refuse to mutate without explicit --yes in JSON mode.
			return nil
		}
		if !syncYes {
			fmt.Printf("Apply %d change(s) to volume %s? [y/N] ", pending, shortID(uuid))
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		return applySync(client, uuid, actions, !syncNoVerify)
	},
}

// walkLocalDir returns rel-path -> size for every file under dir.
func walkLocalDir(dir string, includeAll bool) (map[string]int64, error) {
	files := map[string]int64{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !includeAll && p != dir && syncSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !includeAll && syncSkipFile(d.Name()) {
			return nil
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		// os.Stat (not d.Info, which is lstat) so a symlink reports its target's
		// size — matching the bytes we actually upload. Broken symlinks are
		// skipped rather than aborting the whole walk.
		fi, ierr := os.Stat(p)
		if ierr != nil {
			return nil
		}
		files[filepath.ToSlash(rel)] = fi.Size()
		return nil
	})
	return files, err
}

// diffSync computes the upload/delete plan. Unlike a size-only diff, same-size
// files are content-hashed (unless sizeOnly) so an equal-length edit is still
// detected — otherwise workers would keep old code (the stale-code trap).
func diffSync(client *api.Client, uuid, localDir, remoteRoot string, local, remote map[string]int64, withDelete, sizeOnly bool) ([]syncAction, error) {
	var actions []syncAction
	remotePath := func(rel string) string {
		if remoteRoot == "" {
			return rel
		}
		return remoteRoot + "/" + rel
	}
	for rel, lsz := range local {
		rsz, exists := remote[rel]
		rpath := remotePath(rel)
		lpath := filepath.Join(localDir, filepath.FromSlash(rel))
		changed, reason, err := remoteDiffersFromLocal(client, uuid, rpath, lpath, lsz, rsz, exists, sizeOnly)
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		kind := "changed"
		if reason == "new" {
			kind = "new"
		}
		actions = append(actions, syncAction{rel, lpath, rpath, kind, lsz, rsz})
	}
	if withDelete {
		for rel, rsz := range remote {
			if _, ok := local[rel]; !ok {
				actions = append(actions, syncAction{rel, "", remotePath(rel), "delete", 0, rsz})
			}
		}
	}
	return actions, nil
}

func applySync(client *api.Client, uuid string, actions []syncAction, verify bool) error {
	// Group uploads by destination directory to minimize requests.
	uploads := map[string][]api.UploadItem{}
	var deletes []syncAction
	for _, a := range actions {
		switch a.Kind {
		case "new", "changed":
			dir := baseDir(a.Remote)
			uploads[dir] = append(uploads[dir], api.UploadItem{LocalPath: a.Local, DestName: baseName(a.Remote)})
		case "delete":
			deletes = append(deletes, a)
		}
	}

	// Uploads before deletes so a mid-run failure never leaves the volume with
	// files deleted but their replacements not yet written.
	uploaded := 0
	for dir, items := range uploads {
		if _, err := client.UploadVolumeFiles(uuid, dir, items); err != nil {
			return fmt.Errorf("upload to %q failed: %w", dir, err)
		}
		// Verify each upload read back byte-identical, so a corrupted upload is
		// caught here rather than silently serving bad code.
		if verify {
			for _, it := range items {
				remote := it.DestName
				if dir != "" {
					remote = dir + "/" + it.DestName
				}
				if err := verifyReadback(client, uuid, remote, it.LocalPath); err != nil {
					return err
				}
			}
		}
		uploaded += len(items)
		if !flagJSON {
			fmt.Fprintf(os.Stderr, "  uploaded %d file(s) -> %s\n", len(items), firstNonEmpty(dir, "/"))
		}
	}
	deleted := 0
	for _, a := range deletes {
		if err := client.DeleteVolumePath(uuid, a.Remote); err != nil {
			return fmt.Errorf("delete %q failed: %w", a.Remote, err)
		}
		deleted++
		if !flagJSON {
			fmt.Fprintf(os.Stderr, "  deleted %s\n", a.Remote)
		}
	}

	if flagJSON {
		output.PrintJSON(map[string]interface{}{"status": "success", "uploaded": uploaded, "deleted": deleted})
		return nil
	}
	output.PrintSuccess(fmt.Sprintf("Synced: %d uploaded, %d deleted", uploaded, deleted))
	return nil
}

func printSyncPlan(actions []syncAction, unchanged, endpoints, replicas int, uuid, remoteRoot string) {
	dest := shortID(uuid)
	if remoteRoot != "" {
		dest += "/" + remoteRoot
	}
	fmt.Printf("Sync plan for volume %s:\n", dest)
	shown := 0
	for _, a := range actions {
		if shown >= 40 {
			fmt.Printf("  ... and %d more\n", len(actions)-shown)
			break
		}
		switch a.Kind {
		case "new":
			fmt.Printf("  %s %s (%s)\n", output.Good("+ new    "), a.Rel, formatFileSize(a.LocalSz))
		case "changed":
			fmt.Printf("  %s %s (%s -> %s)\n", output.Warn("~ changed"), a.Rel, formatFileSize(a.RemoteSz), formatFileSize(a.LocalSz))
		case "delete":
			fmt.Printf("  %s %s\n", output.Warn("- delete "), a.Rel)
		}
		shown++
	}
	fmt.Printf("\n%d new, %d changed, %d unchanged", countKind(actions, "new"), countKind(actions, "changed"), unchanged)
	if d := countKind(actions, "delete"); d > 0 {
		fmt.Printf(", %d to delete", d)
	}
	fmt.Println()
	if endpoints > 0 {
		fmt.Printf("Affected: %d endpoint(s), %d active replica(s) — they keep running old code until restarted (see 'serverless deploy').\n", endpoints, replicas)
	}
}

func countKind(actions []syncAction, kind string) int {
	n := 0
	for _, a := range actions {
		if a.Kind == kind {
			n++
		}
	}
	return n
}

// listVolumeFilesRecursive lists every file under dir, walking up to maxDepth
// directory levels. truncated is true if a subdirectory was skipped because the
// depth limit was reached (so the diff isn't silently incomplete).
func listVolumeFilesRecursive(client *api.Client, uuid, dir string, maxDepth int) (files []api.VolumeFile, truncated bool, err error) {
	resp, err := client.ListVolumeFiles(uuid, dir)
	if err != nil {
		return nil, false, err
	}
	for _, item := range resp.Items {
		if item.IsDirectory {
			if maxDepth <= 0 {
				truncated = true
				continue
			}
			child, childTruncated, cerr := listVolumeFilesRecursive(client, uuid, item.Path, maxDepth-1)
			if cerr != nil {
				return nil, false, cerr
			}
			truncated = truncated || childTruncated
			files = append(files, child...)
			continue
		}
		files = append(files, item)
	}
	return files, truncated, nil
}

func syncSkipDir(name string) bool {
	switch name {
	case ".git", "__pycache__", "node_modules", ".ipynb_checkpoints", ".venv", "venv", ".mypy_cache", ".pytest_cache":
		return true
	}
	return false
}

func syncSkipFile(name string) bool {
	lower := strings.ToLower(name)
	// Never push local secrets to a (possibly shared) volume by default.
	switch lower {
	case ".ds_store", ".env", ".netrc", ".npmrc", ".git-credentials", "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519":
		return true
	}
	if strings.HasPrefix(lower, ".env.") {
		return true
	}
	for _, suffix := range []string{".pyc", ".pyo", ".swp", ".swo", ".pem"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
