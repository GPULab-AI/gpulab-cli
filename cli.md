# GPULab CLI - Detailed Implementation Plan

## Overview

A Go CLI tool (`gpulab`) that provides full control over GPULab containers — deploy, manage, SSH, view logs, edit files — all from the terminal. Built for developers and AI agents who need programmatic, closed-loop access to GPU containers without a browser.

**Key Goal**: Give 100% container access via CLI so AI agents (Claude Code, Cursor, etc.) can deploy containers, check logs, SSH in, edit files, and iterate — creating a closed-loop development system.

---

## Architecture

```
┌──────────────────┐         ┌──────────────────┐         ┌──────────────────┐
│   gpulab CLI     │────────►│  gpulab-v2 API   │────────►│  system-server   │
│   (Go binary)    │  HTTPS  │  (Laravel cloud) │  HTTP   │  (FastAPI on GPU │
│   user's machine │◄────────│  gpulab.ai/api   │◄────────│   servers)       │
└──────────────────┘         └──────────────────┘         └──────────────────┘
                                                                    │
                                                           ┌────────┴────────┐
                                                           │  Docker Engine  │
                                                           │  + ttyd         │
                                                           │  + NFS/MooseFS  │
                                                           └─────────────────┘
```

The CLI talks **only** to `gpulab.ai/api` (the Laravel backend). It never communicates directly with system-servers. All operations are proxied through the Laravel API layer, which handles auth, resource allocation, and server routing.

---

## Phase 0: Backend API Extensions (Laravel - gpulab-v2)

The current API (`/api/v1/`) only has 3 container endpoints. We need to add many more for CLI use. All new endpoints go under the existing `ApiKeyAuthMiddleware` group.

### New API Endpoints Needed

Add these to `routes/api.php` inside the `Route::prefix('v1')->middleware(ApiKeyAuthMiddleware::class)` group:

```
GET    /v1/containers/{uuid}              → Single container details
POST   /v1/containers/{uuid}/stop         → Stop container
POST   /v1/containers/{uuid}/start        → Start stopped container
POST   /v1/containers/{uuid}/restart      → Restart container
POST   /v1/containers/{uuid}/redeploy     → Redeploy failed container
GET    /v1/containers/{uuid}/logs         → Get container logs (runtime)
GET    /v1/containers/{uuid}/logs/deploy  → Get deployment logs
GET    /v1/containers/{uuid}/stats        → Get container resource stats
POST   /v1/containers/{uuid}/terminal     → Start terminal session, return connection info
POST   /v1/containers/{uuid}/exec        → Execute single command in container (NEW)

GET    /v1/templates                      → List available templates
GET    /v1/templates/{uuid}               → Get template details

GET    /v1/gpus/types                     → List all GPU types with pricing

GET    /v1/volumes                        → List network volumes
POST   /v1/volumes                        → Create network volume
GET    /v1/volumes/{uuid}                 → Get volume details
PUT    /v1/volumes/{uuid}                 → Update volume (resize)
DELETE /v1/volumes/{uuid}                 → Delete volume

GET    /v1/ssh-keys                       → List SSH keys
POST   /v1/ssh-keys                       → Add SSH key
DELETE /v1/ssh-keys/{id}                  → Remove SSH key

GET    /v1/account                        → Get current user info, billing, etc.
```

### Critical New Endpoint: Container Exec

This is the most important new endpoint for AI agent use. It executes a single command inside a container and returns stdout/stderr.

**How it works:**
1. CLI sends `POST /v1/containers/{uuid}/exec` with `{"command": "cat /workspace/train.py"}`
2. Laravel finds the container's server
3. Laravel calls system-server's `/execute-docker-command` with: `docker exec {container_uuid} sh -c '{command}'`
4. System-server runs the command, captures output
5. System-server returns output to Laravel via webhook OR we add a new synchronous exec endpoint to system-server
6. Laravel returns the output to CLI

**Alternative approach (simpler, recommended):**
Add a new synchronous endpoint to system-server:
```
POST /exec-command
{
  "container_uuid": "xxx",
  "command": "ls -la /workspace",
  "timeout": 30
}
→ {"stdout": "...", "stderr": "...", "exit_code": 0}
```

Then Laravel proxies this:
```
POST /v1/containers/{uuid}/exec
{
  "command": "ls -la /workspace",
  "timeout": 30
}
→ {"stdout": "...", "stderr": "...", "exit_code": 0}
```

### System Server Changes (gpu-lab-system-server)

Add one new endpoint to `app/Docker/dockerController.py`:

```python
@router.post("/exec-command")
async def exec_command(request: ExecCommandRequest):
    """Execute a command in a container synchronously and return output."""
    # docker exec {container_uuid} sh -c '{command}'
    # Capture stdout, stderr, exit code
    # Return synchronously (with timeout)
```

This is all that's needed on system-server side. Everything else already exists.

---

## Phase 1: Project Setup & Core Infrastructure

### 1.1 Go Project Structure

```
gpulab-cli/
├── cmd/
│   └── gpulab/
│       └── main.go                 # Entry point
├── internal/
│   ├── api/
│   │   ├── client.go               # HTTP client for gpulab.ai API
│   │   ├── containers.go           # Container API methods
│   │   ├── templates.go            # Template API methods
│   │   ├── gpus.go                 # GPU API methods
│   │   ├── volumes.go              # Volume API methods
│   │   ├── sshkeys.go              # SSH key API methods
│   │   └── account.go              # Account API methods
│   ├── config/
│   │   └── config.go               # Config file management (~/.gpulab/config.json)
│   ├── commands/
│   │   ├── auth.go                 # login, logout, whoami
│   │   ├── containers.go           # deploy, ls, inspect, stop, start, restart, rm
│   │   ├── logs.go                 # logs (runtime + deploy)
│   │   ├── exec.go                 # exec, ssh
│   │   ├── templates.go            # template list, template info
│   │   ├── gpus.go                 # gpu list, gpu types
│   │   ├── volumes.go              # volume create, ls, resize, rm
│   │   ├── sshkeys.go              # ssh-key add, ls, rm
│   │   └── version.go              # version
│   ├── terminal/
│   │   └── websocket.go            # WebSocket terminal client for SSH
│   └── output/
│       ├── table.go                # Table formatted output
│       └── json.go                 # JSON output mode
├── scripts/
│   └── install.sh                  # curl | bash installer
├── .goreleaser.yml                 # Cross-platform release builds
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 1.2 Dependencies

```go
// go.mod
module github.com/gpulab/gpulab-cli

go 1.22

require (
    github.com/spf13/cobra v1.8.0      // CLI framework
    github.com/gorilla/websocket v1.5.1  // WebSocket for terminal
    github.com/fatih/color v1.16.0       // Colored output
    github.com/olekukonez/tablewriter v0.0.5  // Table output
    github.com/briandowns/spinner v1.23.0     // Loading spinners
)
```

### 1.3 Config File

Location: `~/.gpulab/config.json`

```json
{
  "api_key": "gpulab_abc12345_secret_key_here",
  "api_url": "https://gpulab.ai/api",
  "default_output": "table",
  "default_gpu_type": "NVIDIA GeForce RTX 4090"
}
```

### 1.4 API Client

Core HTTP client that handles:
- API key auth via `api-key` header (matching existing `ApiKeyAuthMiddleware`)
- Base URL configuration
- JSON request/response marshaling
- Error handling with user-friendly messages
- Retry logic for transient failures
- `--json` flag support for machine-readable output (for AI agents)

---

## Phase 2: Authentication & Basic Commands

### 2.1 Auth Commands

```bash
# Set API key (stored in ~/.gpulab/config.json)
gpulab auth login
# Prompts: "Enter your API key: gpulab_xxxx_yyyy"
# Validates key by calling GET /v1/account
# Stores key in config file

gpulab auth login --api-key gpulab_xxxx_yyyy
# Non-interactive mode (for CI/CD and AI agents)

gpulab auth logout
# Removes API key from config

gpulab auth whoami
# Shows current user info from GET /v1/account
# Output: "Logged in as: user@example.com (Team: MyTeam)"

gpulab auth status
# Shows auth status and API connectivity
```

### 2.2 Version Command

```bash
gpulab version
# Output: "gpulab v1.0.0 (darwin/arm64)"
```

---

## Phase 3: Container Lifecycle Commands

### 3.1 Deploy (Create) Container

```bash
# Full form
gpulab deploy \
  --name "my-training-job" \
  --gpu-type "NVIDIA GeForce RTX 4090" \
  --template pytorch-2.1 \
  --ports 8080,8888 \
  --env BATCH_SIZE=32 \
  --env LEARNING_RATE=0.001 \
  --volume my-workspace \
  --mount-path /workspace \
  --memory 32 \
  --command "python train.py"

# Minimal (uses defaults from template)
gpulab deploy --name "quick-job" --gpu-type "RTX 4090" --template pytorch

# With --wait flag (blocks until RUNNING or FAILED)
gpulab deploy --name "job" --template pytorch --gpu-type "RTX 4090" --wait

# JSON output for AI agents
gpulab deploy --name "job" --template pytorch --gpu-type "RTX 4090" --json
```

**Implementation:**
1. Validate inputs locally first
2. Look up template UUID from name via `GET /v1/templates?name=pytorch`
3. Look up volume UUID from name via `GET /v1/volumes?name=my-workspace`
4. `POST /v1/containers` with full payload
5. Show spinner while deploying
6. If `--wait`: poll `GET /v1/containers/{uuid}` every 3 seconds until status changes
7. Print result with container UUID, status, and access URLs

**Output:**
```
✓ Container deployed successfully

  Name:    my-training-job
  UUID:    abc-123-def-456
  Status:  deploying
  GPU:     NVIDIA GeForce RTX 4090
  Ports:   8080 → https://abc-123-def-456-8080.proxy.gpulab.ai
           8888 → https://abc-123-def-456-8888.proxy.gpulab.ai

  Use 'gpulab logs abc-123' to view deployment progress
  Use 'gpulab ssh abc-123' to connect when ready
```

### 3.2 List Containers

```bash
gpulab ls
# or
gpulab containers ls

# With status filter
gpulab ls --status running
gpulab ls --status stopped

# JSON output
gpulab ls --json
```

**Output:**
```
NAME                 UUID          STATUS    GPU            UPTIME
my-training-job      abc-123...    running   RTX 4090       2h 15m
test-server          def-456...    stopped   RTX 3090       -
data-processing      ghi-789...    deploying A100           -
```

### 3.3 Inspect Container

```bash
gpulab inspect abc-123
# or
gpulab containers inspect abc-123

# JSON output
gpulab inspect abc-123 --json
```

**Output:**
```
Container: my-training-job
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
UUID:        abc-123-def-456-ghi-789
Status:      running
Type:        GPU
GPU:         NVIDIA GeForce RTX 4090 (24GB)
Memory:      32 GB
CPU Cores:   12
Created:     2026-03-01 10:00:00 UTC
Uptime:      2h 15m 30s

Ports:
  8080 → https://abc-123-def-456-8080.proxy.gpulab.ai
  8888 → https://abc-123-def-456-8888.proxy.gpulab.ai

Environment:
  BATCH_SIZE=32
  LEARNING_RATE=0.001

Volume: my-workspace → /workspace
Terminal: https://abc-123-def-456-terminal.proxy.gpulab.ai
```

### 3.4 Stop / Start / Restart / Redeploy / Delete

```bash
gpulab stop abc-123           # POST /v1/containers/{uuid}/stop
gpulab start abc-123          # POST /v1/containers/{uuid}/start
gpulab restart abc-123        # POST /v1/containers/{uuid}/restart
gpulab redeploy abc-123       # POST /v1/containers/{uuid}/redeploy
gpulab rm abc-123             # DELETE /v1/containers/{uuid}
gpulab rm abc-123 --force     # Skip confirmation prompt
```

All commands accept UUID prefix matching (first 6+ chars are enough if unique).

---

## Phase 4: Logs & Monitoring

### 4.1 Runtime Logs

```bash
# Get last 100 lines
gpulab logs abc-123

# Get last N lines
gpulab logs abc-123 --tail 500

# Follow logs (streaming, like docker logs -f)
gpulab logs abc-123 --follow

# With timestamps
gpulab logs abc-123 --timestamps

# Stderr only
gpulab logs abc-123 --stderr

# JSON output (for AI agent parsing)
gpulab logs abc-123 --json
```

**Implementation:**
- `GET /v1/containers/{uuid}/logs?tail=100&timestamps=true`
- For `--follow`: Poll every 2 seconds with `since` timestamp, append new lines
- Returns demultiplexed stdout/stderr with color coding

### 4.2 Deployment Logs

```bash
gpulab logs abc-123 --deploy
# Shows the deployment/build logs (image pull, container start)
# GET /v1/containers/{uuid}/logs/deploy
```

### 4.3 Container Stats

```bash
gpulab stats abc-123
# GET /v1/containers/{uuid}/stats

# Continuous monitoring (refresh every 2s)
gpulab stats abc-123 --watch
```

**Output:**
```
Container: my-training-job (abc-123)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
CPU:     45.2%   ████████░░░░░░░░░░░░  (12 cores)
Memory:  18.5 GB / 32.0 GB  ███████████░░░░░░░░░
GPU Mem: 20.1 GB / 24.0 GB  ████████████████░░░░
GPU Temp: 72°C

Network:  ↓ 1.2 GB  ↑ 345 MB
Block IO: R 5.1 GB  W 2.3 GB
PIDs:     42
```

---

## Phase 5: Container Access (Critical for AI Agents)

### 5.1 SSH / Interactive Terminal

```bash
# Open interactive shell in container
gpulab ssh abc-123
# or
gpulab exec abc-123 --interactive
```

**Implementation — Two Approaches:**

#### Approach A: WebSocket-based terminal (Recommended)

1. CLI calls `POST /v1/containers/{uuid}/terminal` to start ttyd
2. Backend calls system-server's `/create_terminal`, gets back terminal port
3. Backend returns WebSocket URL: `wss://abc-123-terminal.proxy.gpulab.ai`
4. CLI opens WebSocket connection to this URL
5. CLI uses Go's `golang.org/x/term` to put local terminal in raw mode
6. Bidirectional streaming: local stdin → WebSocket → ttyd → container shell
7. Container output → WebSocket → local stdout
8. On Ctrl+D or exit: close WebSocket, restore terminal

This is the most reliable approach since ttyd + WebSocket is already working for browser access.

#### Approach B: Docker exec proxy (Fallback)

If WebSocket is complex, we can add a simple TCP proxy:
1. System-server opens a `docker exec -it` session
2. Exposes it on a port
3. CLI connects via TCP

**Approach A is preferred** since the infrastructure (ttyd, gateway proxy, WebSocket) already exists.

### 5.2 Non-Interactive Exec (Most important for AI agents)

```bash
# Run single command, get output
gpulab exec abc-123 -- ls -la /workspace
gpulab exec abc-123 -- cat /workspace/train.py
gpulab exec abc-123 -- python -c "print('hello')"
gpulab exec abc-123 -- nvidia-smi
gpulab exec abc-123 -- pip list

# With JSON output
gpulab exec abc-123 -- cat /workspace/results.json --json

# With timeout
gpulab exec abc-123 --timeout 60 -- python train.py
```

**Implementation:**
1. `POST /v1/containers/{uuid}/exec` with `{"command": "ls -la /workspace", "timeout": 30}`
2. Laravel proxies to system-server's new `/exec-command` endpoint
3. System-server runs `docker exec {container_uuid} sh -c '{command}'`
4. Returns `{"stdout": "...", "stderr": "...", "exit_code": 0}`
5. CLI prints stdout to stdout, stderr to stderr, exits with the container's exit code

**This is THE critical command for AI agents.** It enables:
- Reading files: `gpulab exec abc-123 -- cat /workspace/model.py`
- Writing files: `gpulab exec abc-123 -- sh -c 'cat > /workspace/config.yaml << EOF\n...\nEOF'`
- Running code: `gpulab exec abc-123 -- python train.py`
- Installing packages: `gpulab exec abc-123 -- pip install torch`
- Checking GPU: `gpulab exec abc-123 -- nvidia-smi`
- Debugging: `gpulab exec abc-123 -- tail -100 /workspace/output.log`

### 5.3 File Transfer (via exec)

```bash
# Upload file to container
gpulab cp ./local-file.py abc-123:/workspace/file.py

# Download file from container
gpulab cp abc-123:/workspace/results.json ./results.json

# Upload directory
gpulab cp ./src/ abc-123:/workspace/src/
```

**Implementation:**
- Upload: Read local file, base64 encode, exec `echo '<base64>' | base64 -d > /path/file` in container
- For large files: Use the system-server's existing `/files/upload/{volume_uuid}` endpoint via a new Laravel proxy
- Download: exec `cat /path/file` or `base64 /path/file` and decode locally
- For large files: Use system-server's `/files/download/{volume_uuid}` endpoint via a new Laravel proxy

**Alternative for volumes (better for large files):**
If the container has a network volume, we can use the file operations API:
```
GET  /v1/volumes/{uuid}/files?path=/workspace       → list files
GET  /v1/volumes/{uuid}/files/content?path=/file.py  → read file
PUT  /v1/volumes/{uuid}/files/content               → write file
POST /v1/volumes/{uuid}/files/upload                → upload file
GET  /v1/volumes/{uuid}/files/download?path=/file   → download file
```

These would proxy to the system-server's existing FileOps endpoints.

---

## Phase 6: Resource Management Commands

### 6.1 GPU Commands

```bash
# List available GPUs
gpulab gpus
# or
gpulab gpus available

# Filter by type
gpulab gpus --type "RTX 4090"
gpulab gpus --min-memory 24000

# List GPU types with pricing
gpulab gpus types
```

**Output:**
```
GPU TYPE                    AVAILABLE    MEMORY    PRICE/HR
NVIDIA GeForce RTX 3090     8           24 GB     $0.99
NVIDIA GeForce RTX 4090     12          24 GB     $1.49
NVIDIA A100                 4           40 GB     $2.49
NVIDIA H100                 2           80 GB     $4.99
```

### 6.2 Template Commands

```bash
# List available templates
gpulab templates
gpulab templates --type gpu
gpulab templates --type cpu

# Get template details
gpulab templates info pytorch-2.1

# JSON output
gpulab templates --json
```

**Output:**
```
NAME              TYPE   IMAGE                         PORTS        MEMORY
pytorch-2.1       GPU    pytorch/pytorch:2.1.0-cuda    8888         32 GB
tensorflow-2.15   GPU    tensorflow/tensorflow:2.15    8888,6006    32 GB
ubuntu-22.04      CPU    ubuntu:22.04                  -            4 GB
jupyter-minimal   GPU    jupyter/minimal-notebook      8888         16 GB
```

### 6.3 Volume Commands

```bash
# List volumes
gpulab volumes

# Create volume
gpulab volumes create --name my-data --size 100
# POST /v1/volumes

# Get volume info
gpulab volumes info my-data

# Resize volume
gpulab volumes resize my-data --size 200

# Delete volume
gpulab volumes rm my-data
gpulab volumes rm my-data --force

# List files in volume (via proxy to system-server FileOps)
gpulab volumes ls my-data --path /training-data
```

**Output:**
```
NAME              SIZE     USED      STATUS     CONTAINERS
my-workspace      100 GB   45.2 GB   created    my-training-job
shared-data       500 GB   312 GB    created    data-processor, analyzer
```

### 6.4 SSH Key Commands

```bash
gpulab ssh-keys
gpulab ssh-keys add --name "my-laptop" --key "ssh-rsa AAAA..."
gpulab ssh-keys add --name "my-laptop" --key-file ~/.ssh/id_rsa.pub
gpulab ssh-keys rm my-laptop
```

---

## Phase 7: Global Flags & Output Modes

### 7.1 Global Flags

```
--json          Output in JSON format (for AI agent parsing)
--quiet / -q    Minimal output (just UUIDs/status)
--no-color      Disable colored output
--api-key       Override API key for this command
--api-url       Override API URL (for testing)
--debug         Show HTTP request/response details
--timeout       HTTP request timeout (default: 30s)
```

### 7.2 Output Modes

All commands support three output modes:

1. **Table mode** (default, human-friendly): Formatted tables with colors
2. **JSON mode** (`--json`): Machine-readable JSON for AI agents and scripts
3. **Quiet mode** (`-q`): Just essential info (UUID, status code)

**JSON mode example:**
```bash
gpulab ls --json
```
```json
[
  {
    "uuid": "abc-123-def-456",
    "name": "my-training-job",
    "status": "running",
    "gpu_type": "NVIDIA GeForce RTX 4090",
    "uptime": 8100,
    "ports": {"8080": "https://abc-123-8080.proxy.gpulab.ai"},
    "created_at": "2026-03-01T10:00:00Z"
  }
]
```

---

## Phase 8: Installation & Distribution

### 8.1 Install Script (`install.sh`)

Hosted at `https://gpulab.ai/cli.sh`

```bash
curl -fsSL https://gpulab.ai/cli.sh | bash
```

**Script behavior:**
1. Detect OS (darwin/linux) and arch (amd64/arm64)
2. Download correct binary from GitHub releases
3. Install to `/usr/local/bin/gpulab` (or `~/.local/bin/gpulab`)
4. Verify checksum
5. Print success message with `gpulab auth login` next step

```bash
#!/bin/bash
set -euo pipefail

VERSION="${GPULAB_VERSION:-latest}"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL https://api.github.com/repos/gpulab/gpulab-cli/releases/latest | grep tag_name | cut -d'"' -f4)
fi

DOWNLOAD_URL="https://github.com/gpulab/gpulab-cli/releases/download/${VERSION}/gpulab_${OS}_${ARCH}.tar.gz"

echo "Downloading gpulab ${VERSION} for ${OS}/${ARCH}..."
curl -fsSL "$DOWNLOAD_URL" -o /tmp/gpulab.tar.gz
tar -xzf /tmp/gpulab.tar.gz -C /tmp

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

mv /tmp/gpulab "$INSTALL_DIR/gpulab"
chmod +x "$INSTALL_DIR/gpulab"
rm /tmp/gpulab.tar.gz

echo ""
echo "✓ gpulab installed to $INSTALL_DIR/gpulab"
echo ""
echo "Get started:"
echo "  gpulab auth login"
echo ""
```

### 8.2 GoReleaser Config

`.goreleaser.yml` builds for:
- `darwin/amd64` (Intel Mac)
- `darwin/arm64` (Apple Silicon)
- `linux/amd64`
- `linux/arm64`

```yaml
project_name: gpulab

builds:
  - id: gpulab
    main: ./cmd/gpulab
    binary: gpulab
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.ShortCommit}}
      - -X main.date={{.Date}}

archives:
  - id: gpulab
    format: tar.gz
    name_template: "gpulab_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

release:
  github:
    owner: gpulab
    name: gpulab-cli
```

### 8.3 Makefile

```makefile
.PHONY: build test clean install

VERSION ?= dev
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/gpulab ./cmd/gpulab

install:
	go install $(LDFLAGS) ./cmd/gpulab

test:
	go test ./... -v

clean:
	rm -rf bin/

release:
	goreleaser release --clean
```

---

## Phase 9: WebSocket Terminal Implementation

### How `gpulab ssh` works under the hood

```
┌─────────┐     WebSocket      ┌───────────────┐     WebSocket     ┌──────────┐
│ gpulab   │ ◄─────────────────►│ Gateway Proxy │◄─────────────────►│   ttyd   │
│ CLI      │    (wss://)        │ (OpenResty)   │    (ws://)        │ process  │
│          │                    │               │                    │          │
│ stdin ──►│                    │               │                    │──► shell │
│ stdout◄──│                    │               │                    │◄── shell │
└─────────┘                    └───────────────┘                    └──────────┘
     ▲                                                                    │
     │                                                               docker exec
     │                                                               -it /bin/sh
  local terminal                                                          │
  (raw mode)                                                        ┌──────────┐
                                                                    │Container │
                                                                    └──────────┘
```

**Steps:**
1. `gpulab ssh abc-123` → CLI calls `POST /v1/containers/{uuid}/terminal`
2. If terminal not already started, backend starts ttyd via system-server
3. Backend returns `{"websocket_url": "wss://abc-123-terminal.proxy.gpulab.ai/ws"}`
4. CLI connects to WebSocket URL
5. CLI puts local terminal in raw mode (`golang.org/x/term`)
6. Goroutine 1: Read local stdin → send to WebSocket
7. Goroutine 2: Read from WebSocket → write to local stdout
8. On disconnect/Ctrl+D: restore terminal, close WebSocket

**Key Go packages:**
- `github.com/gorilla/websocket` — WebSocket client
- `golang.org/x/term` — Raw terminal mode
- `os/signal` — Handle SIGWINCH (terminal resize)

**Terminal resize handling:**
- Listen for SIGWINCH signals
- On resize, send terminal dimensions via WebSocket control message
- ttyd supports resize via JSON message: `{"type": "resize", "cols": 120, "rows": 40}`

---

## Phase 10: AI Agent Integration

### Design for AI Agent Use

The CLI must be optimized for programmatic use by AI agents (Claude Code, Cursor, Aider, etc.):

1. **`--json` flag everywhere**: All commands output parseable JSON
2. **Non-zero exit codes**: Failed operations return non-zero exit codes
3. **stderr for errors**: Errors go to stderr, data goes to stdout
4. **Non-interactive by default**: `--api-key` flag avoids login prompt
5. **`gpulab exec`**: The killer feature — run any command in any container

### Example AI Agent Workflow

```bash
# AI Agent deploys a container
CONTAINER=$(gpulab deploy --name "training" --template pytorch --gpu-type "RTX 4090" --wait --json | jq -r '.container_id')

# Check it's running
gpulab inspect $CONTAINER --json | jq '.status'

# Upload training code
gpulab exec $CONTAINER -- sh -c 'cat > /workspace/train.py << "PYEOF"
import torch
# ... training code ...
PYEOF'

# Run training
gpulab exec $CONTAINER -- python /workspace/train.py 2>&1

# Check GPU usage
gpulab exec $CONTAINER -- nvidia-smi

# Read results
gpulab exec $CONTAINER -- cat /workspace/results.json

# View logs
gpulab logs $CONTAINER --tail 50

# Stop when done
gpulab stop $CONTAINER
```

### Environment Variable Auth

For CI/CD and AI agents:
```bash
export GPULAB_API_KEY=gpulab_xxxx_yyyy
gpulab ls  # Uses env var, no config file needed
```

Priority: `--api-key` flag > `GPULAB_API_KEY` env var > `~/.gpulab/config.json`

---

## Implementation Order

### Sprint 1: Foundation (Week 1)
1. Go project setup (go.mod, structure, Makefile)
2. API client with auth
3. Config file management
4. `gpulab auth login/logout/whoami`
5. `gpulab version`

### Sprint 2: Container Basics (Week 1-2)
1. **Backend**: Add missing API endpoints (show, stop, start, restart, logs, stats)
2. `gpulab ls` (list containers)
3. `gpulab inspect` (container details)
4. `gpulab deploy` (create container)
5. `gpulab stop/start/restart/rm`

### Sprint 3: Logs & Exec (Week 2-3)
1. **Backend**: Add `/v1/containers/{uuid}/exec` endpoint
2. **System-server**: Add `/exec-command` synchronous endpoint
3. `gpulab logs` (with --follow, --tail, --deploy)
4. `gpulab exec` (non-interactive command execution)
5. `gpulab stats`

### Sprint 4: SSH Terminal (Week 3-4)
1. WebSocket terminal client
2. `gpulab ssh` (interactive terminal)
3. Terminal resize handling
4. Connection recovery/reconnect

### Sprint 5: Resource Management (Week 4)
1. **Backend**: Add template, volume, GPU type API endpoints
2. `gpulab templates`
3. `gpulab gpus`
4. `gpulab volumes` (CRUD)
5. `gpulab ssh-keys`

### Sprint 6: File Transfer & Polish (Week 4-5)
1. `gpulab cp` (file upload/download)
2. `--json` output for all commands
3. Install script
4. GoReleaser setup
5. README and docs

### Sprint 7: Testing & Release (Week 5)
1. Unit tests for API client
2. Integration tests against test environment
3. First release build
4. Upload install.sh to gpulab.ai/cli.sh
5. Documentation

---

## Complete Command Reference

```
gpulab auth login [--api-key KEY]      Set up authentication
gpulab auth logout                     Remove stored credentials
gpulab auth whoami                     Show current user
gpulab auth status                     Check auth & API connectivity

gpulab deploy [flags]                  Deploy new container
gpulab ls [--status STATUS]            List containers
gpulab inspect UUID                    Show container details
gpulab stop UUID                       Stop running container
gpulab start UUID                      Start stopped container
gpulab restart UUID                    Restart container
gpulab redeploy UUID                   Redeploy failed container
gpulab rm UUID [--force]               Delete container

gpulab logs UUID [--follow] [--tail N] View container logs
gpulab logs UUID --deploy              View deployment logs
gpulab stats UUID [--watch]            View container resource stats

gpulab ssh UUID                        Interactive shell in container
gpulab exec UUID -- COMMAND            Run command in container
gpulab cp SRC DST                      Copy files to/from container

gpulab gpus                            List available GPUs
gpulab gpus types                      List GPU types with pricing
gpulab templates [--type gpu|cpu]      List container templates
gpulab templates info NAME             Show template details

gpulab volumes                         List network volumes
gpulab volumes create [flags]          Create network volume
gpulab volumes info UUID               Show volume details
gpulab volumes resize UUID --size N    Resize volume
gpulab volumes rm UUID                 Delete volume

gpulab ssh-keys                        List SSH keys
gpulab ssh-keys add [flags]            Add SSH key
gpulab ssh-keys rm NAME                Remove SSH key

gpulab version                         Show CLI version
gpulab help [command]                  Show help

Global flags:
  --json        JSON output
  --quiet       Minimal output
  --no-color    Disable colors
  --api-key     Override API key
  --api-url     Override API URL
  --debug       Debug mode
  --timeout     Request timeout
```

---

## Key Technical Decisions

### 1. Go over Python/Node
- Single binary, no runtime dependencies
- Cross-compile to mac/linux easily
- Fast startup time (matters for AI agents calling CLI repeatedly)
- Native terminal handling

### 2. Cobra for CLI framework
- Industry standard (used by kubectl, gh, docker CLI)
- Built-in help, completion, subcommands
- Well-documented, large community

### 3. WebSocket for SSH (not raw TCP)
- Infrastructure already exists (ttyd + gateway proxy)
- Works through firewalls and HTTPS
- No need for SSH keys or port forwarding
- Gateway already handles TLS termination

### 4. Exec via backend proxy (not direct to system-server)
- Maintains single auth point (API key → Laravel)
- No need to expose system-server to internet
- Audit logging in Laravel
- Consistent error handling

### 5. UUID prefix matching
- Users can type `gpulab ssh abc-12` instead of full UUID
- CLI checks if prefix is unique, errors if ambiguous
- Similar to Docker CLI behavior

---

## Security Considerations

1. **API key storage**: `~/.gpulab/config.json` with `0600` permissions
2. **No secrets in commands**: API key never passed as command arg (visible in `ps`)
3. **TLS everywhere**: All communication over HTTPS/WSS
4. **Exec command sanitization**: Backend must sanitize exec commands to prevent escape
5. **Timeout enforcement**: All exec commands have configurable timeout (default 30s, max 300s)
6. **Rate limiting**: Backend should rate-limit exec calls per user
7. **File transfer limits**: Max file size for cp operations (100MB via exec, larger via volume API)

---

## Backend Changes Summary

### gpulab-v2 (Laravel) — New Code Needed

**New API Controller methods** (add to `Api\V1\ContainerController` or new controllers):
- `show(uuid)` — single container details
- `stop(uuid)` — stop container
- `start(uuid)` — start container
- `restart(uuid)` — restart container
- `redeploy(uuid)` — redeploy container
- `logs(uuid)` — get runtime logs (proxy to Docker Engine API)
- `deploymentLogs(uuid)` — get deployment logs
- `stats(uuid)` — get container stats
- `exec(uuid)` — execute command in container (proxy to system-server)
- `startTerminal(uuid)` — start terminal, return WebSocket URL

**New API Controllers:**
- `Api\V1\TemplateController` — list/show templates
- `Api\V1\VolumeController` — CRUD volumes
- `Api\V1\GpuController` — list GPU types
- `Api\V1\SshKeyController` — CRUD SSH keys
- `Api\V1\AccountController` — user info

**Estimated new routes**: ~20 endpoints

### gpu-lab-system-server (FastAPI) — New Code Needed

**One new endpoint:**
- `POST /exec-command` — synchronous command execution in container

**Estimated effort**: ~50 lines of Python

---

## Testing Strategy

### Unit Tests
- API client: Mock HTTP responses, test all methods
- Config: Test read/write/merge of config files
- Output: Test table/JSON formatting
- Commands: Test argument parsing and validation

### Integration Tests
- Use test environment (`./scripts/test-env.sh`)
- Test full flow: login → deploy → logs → exec → stop → rm
- Test error handling: invalid API key, container not found, server offline

### Manual Testing
- Test on macOS (arm64 + amd64)
- Test on Ubuntu Linux
- Test install script
- Test with actual GPU containers on production
