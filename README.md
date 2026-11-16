# GPULab CLI

Command-line interface for [GPULab](https://gpulab.ai) — deploy, manage, and interact with GPU containers from your terminal.

## Install

```bash
# macOS / Linux
curl -fsSL https://gpulab.ai/cli.sh | sh

# Or build from source
go install github.com/GPULab-AI/gpulab-cli/cmd/gpulab@latest

# Or clone and build
git clone https://github.com/GPULab-AI/gpulab-cli.git
cd gpulab-cli
make install
```

## Quick Start

```bash
# Authenticate
gpulab auth login --api-key gpulab_xxx

# List containers
gpulab ls

# Deploy a container
gpulab deploy --name my-project --template pytorch --gpu-type "RTX 4090" --wait

# Execute commands
gpulab exec <uuid> -- nvidia-smi
gpulab exec <uuid> -- python train.py

# View logs
gpulab logs <uuid> --follow

# SSH into container
gpulab ssh <uuid>

# Stop and delete
gpulab stop <uuid>
gpulab rm <uuid> --force
```

## Deploy

The `deploy` command supports Docker-style syntax for ports, environment variables, and commands.

### Ports

```bash
# Comma-separated list
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  --ports "8080,3000,6006"

# Docker-style repeatable flag
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  -p 8080 -p 3000 -p 6006

# host:container syntax (host port is auto-assigned by GPULab)
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  -p 8080:80 -p 3000:3000

# Both styles can be combined
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  --ports "8080" -p 3000 -p 6006
```

### Startup Command

```bash
# Using --command flag
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  --command "python train.py"

# Docker-style — everything after -- becomes the command
# Useful when the command has its own flags
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  --wait -- python train.py --epochs 100 --lr 0.001

gpulab deploy --name sglang --template pytorch --gpu-type "RTX 4090" \
  -- sglang.deploy --model meta-llama/Llama-3-8B --tp 4
```

### Environment Variables

```bash
# Set variables explicitly (Docker-style)
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  -e HF_TOKEN=hf_xxx \
  -e WANDB_API_KEY=abc123 \
  -e MODEL_NAME=meta-llama/Llama-3-8B

# Inherit from host environment (just pass the key name)
export HF_TOKEN=hf_xxx
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  -e HF_TOKEN -e WANDB_API_KEY

# Load from an env file
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  --env-file .env

# Combine all three — -e flags override --env-file values
gpulab deploy --name test --template pytorch --gpu-type "RTX 4090" \
  --env-file .env \
  -e HF_TOKEN=hf_override \
  -e WANDB_API_KEY
```

The `--env-file` format supports standard `.env` syntax:

```bash
# .env
HF_TOKEN=hf_xxx
WANDB_API_KEY=abc123
MODEL_NAME="meta-llama/Llama-3-8B"
export CUDA_VISIBLE_DEVICES=0,1
# Comments and blank lines are ignored
```

### Full Deploy Example

```bash
gpulab deploy \
  --name training-run \
  --template pytorch \
  --gpu-type 1,3 \
  --memory 32 \
  --ports "8080,6006" \
  --env-file .env \
  -e HF_TOKEN \
  -e RUN_ID=experiment-42 \
  --volume my-volume-uuid \
  --wait \
  -- python train.py --epochs 100 --lr 0.001 --batch-size 32
```

## Serverless GPUs

Serverless endpoints use the same API key auth as containers.

```bash
# See available serverless templates, GPU types, regions, volumes, and policy templates
gpulab serverless options

# Create an endpoint
gpulab serverless create \
  --name llama-api \
  --template pytorch \
  --gpu-type 1,3 \
  --memory 32 \
  --port 8000 \
  --min-replicas 0 \
  --max-replicas 2 \
  --concurrency 1 \
  -e HF_TOKEN \
  --command "python app.py"

# Inspect, invoke, and read logs/history
gpulab serverless inspect llama-api
gpulab serverless invoke llama-api /v1/chat/completions -d '{"prompt":"hello"}' --wait
gpulab serverless requests llama-api
gpulab serverless autoscaling-logs llama-api
gpulab serverless logs llama-api --replica all
gpulab serverless logs llama-api --deploy

# Update or delete
gpulab serverless update llama-api --max-replicas 4 --autoscaling-template pending_requests_linear
gpulab serverless delete llama-api --force
```

### Multiple GPU types (fallbacks)

An endpoint can list up to 5 GPU types in preference order. A new replica goes
on the **first type that has a free GPU**; a replica never mixes types. When load
drops, replicas on fallback types are scaled down first, so the endpoint drifts
back to its preferred GPU. Each replica bills at the price of the GPU it runs on.

```bash
# Look up IDs: names repeat with different VRAM (e.g. two "RTX 4090" rows)
gpulab serverless options

# Preferred RTX 4090 24GB (id 1), fall back to RTX 3090 (id 3)
gpulab serverless update llama-api --gpu-type 1,3

# Edit the list without retyping it
gpulab serverless update llama-api --add-gpu-type "3090 Ti"
gpulab serverless update llama-api --remove-gpu-type 3

# See the order and which GPU each replica landed on
gpulab serverless inspect llama-api
gpulab serverless replicas llama-api          # GPU column per replica
```

- Names resolve as: ID → full name → short name (`RTX 3090`) → whole-word
  suffix (`3090` is the 3090, not the 3090 Ti) → unique substring. An
  ambiguous name fails and lists the IDs to choose from.
- **Test the image on every listed GPU first.** Some kernels need a newer
  architecture (for example FP8 needs Ada/sm_89; the RTX 3090 is sm_86). Each
  replica gets `GPULAB_GPU_TYPE_ID`, `GPULAB_GPU_NAME` and `GPULAB_GPU_VRAM_MB`
  so the worker can pick a code path per GPU.
- Running replicas keep their GPU when you change the list; new replicas follow
  the new order.
- `--json` output includes `gpu_types` (ordered) and per-replica `gpu_type_id` /
  `gpu_type_name`.

```bash
# Only failed requests (or: in_flight, errors, completed, timed_out, ...)
gpulab serverless requests llama-api --status errors
```

### Deploying code & debugging

The serverless commands are built for closed-loop debugging. Logs are cleaned of
Docker stream framing automatically (no more piping through `strings`), and all
timestamps render in UTC by default (override with `--tz local` or `--tz Asia/Kolkata`).

```bash
# Atomic deploy: upload code → verify byte-identical → rolling restart → smoke test.
# Replaces the manual upload-then-restart dance and prevents the stale-replica trap.
gpulab serverless deploy llama-api ./tasks.py --to server/tasks.py --drain

# One health surface: error rate, 4xx/5xx, latency, queue depth, cold starts,
# scaling ping-pong, and (with --logs) detected tracebacks / CUDA OOM.
# It also reports the metrics the autoscaling policy collects (queue_num,
# queue_time, ...) and counts evaluations that asked for replicas and got none,
# so a queue-backed endpoint starved of GPUs does not read as healthy.
gpulab serverless health llama-api --logs

# Trace one request end to end: queued → started → completed, with the HTTP result.
gpulab serverless trace <request-uuid>

# Replicas with age, and a stale check: flags replicas started before the
# volume's newest code file (⚠ running old code).
gpulab serverless replicas llama-api --stale --code-path server

# Restart: a single replica, all replicas, or a zero-downtime rolling restart.
# --drain finishes in-flight requests before tearing each replica down.
gpulab serverless restart llama-api <replica>          # one replica
gpulab serverless restart llama-api --all --rolling    # rolling, no full outage
gpulab serverless restart llama-api --all --drain      # graceful

# Logs: fleet-wide, filtered, time-bounded, with traceback detection.
gpulab serverless logs llama-api --replica all --since 10m --grep Traceback --detect

# Which endpoints use a volume? (reverse lookup)
gpulab volumes endpoints my-volume
gpulab serverless list --volume my-volume

# Which autoscaling policy is attached? The list API does not return the policy,
# so the SCALING column reads "on:?" until --with-policy loads each endpoint.
# It never reports an unknown policy as an absent one.
gpulab serverless list --with-policy
```

### Capacity: free vs placeable GPUs

`gpulab gpus` reports two counts, and they are not the same number:

- **FREE** — every unassigned, online GPU.
- **PLACEABLE** — the subset the scheduler will actually put work on. A GPU on a
  host that is in maintenance, or that is not a GPU host, is free forever and
  placeable never.

Scale against PLACEABLE. An endpoint's own region and CUDA filters can lower it
further, so treat it as an upper bound for any one endpoint.

```bash
gpulab gpus              # GPU types with FREE and PLACEABLE side by side
gpulab gpus available    # the same split, per GPU currently free
```

> **Note on reloading code.** `serverless restart` / `serverless deploy` reload
> the latest volume code because they re-provision a fresh replica container. The
> container-level `gpulab restart <uuid>` (and the `/restart` HTTP route) do **not**
> reload code — the CLI detects when you point one at a serverless replica and
> redirects you to `serverless restart`. A precise per-worker code fingerprint
> (to catch a worker that kept old code in memory) needs the model image to report
> its loaded version; until then, use `replicas --stale` as the practical signal.

### Syncing a directory to a volume

```bash
# rsync-style: uploads only new/changed files (compared by content hash, so
# equal-length edits are caught), hash-verifies each upload, previews the diff,
# and warns how many endpoints/replicas use the volume (they keep old code until
# restarted).
gpulab volumes sync ./server my-volume server --dry-run   # preview only
gpulab volumes sync ./server my-volume server             # upload changes
gpulab volumes sync ./server my-volume server --size-only # faster, size-only compare
gpulab volumes files upload my-volume ./local.py --to server/tasks.py  # single file, renamed
```

> `volumes sync --delete` refuses to run against an empty local dir (would wipe
> the volume) unless you pass `--allow-empty`, and `--json`/non-interactive runs
> won't mutate the volume without `--yes`.

## Templates (Docker images)

Create and manage reusable container templates. Only `--name` and `--image` are
required; visibility defaults to `private`, container type to `gpu`, disk to 10GB.

```bash
# List, inspect, and discover categories
gpulab templates
gpulab templates info my-template
gpulab templates categories

# Create
gpulab templates create --name web --image nginx:latest --ports 80,443
gpulab templates create --name trainer --image pytorch/pytorch:latest \
  --type gpu --memory 24 --disk 50 --mount-path /workspace \
  -e WANDB_API_KEY=xxx --env-file ./train.env

# Edit (only the flags you pass change) and delete
gpulab templates edit my-template --image nginx:1.27 --visibility public
gpulab templates delete my-template --force
```

The target for `info`/`edit`/`delete` may be a full UUID, a UUID prefix, or the
template name.

### Private images

For images that need a registry login, attach Docker credentials. Set them inline
when creating the template (the credential is created and linked in one call):

```bash
gpulab templates create --name private-trainer --image adhik/private:v1 \
  --registry-username adhik --registry-password dckr_pat_xxx
# --registry defaults to docker.io; pass it for ghcr.io, quay.io, etc.
```

Or manage credentials as reusable, named resources and reference them by ID:

```bash
# Store once (use --password-stdin to keep the token out of shell history)
echo "$DOCKER_TOKEN" | gpulab credentials add --username adhik --password-stdin
gpulab credentials                 # list — shows the ID
gpulab templates create --name app --image adhik/private:v1 --credentials 42
gpulab credentials rm 42           # delete when no longer needed
```

Passwords/tokens are write-only — they are never returned by `credentials list`.

## Commands

| Command | Description |
|---------|-------------|
| `gpulab auth login` | Authenticate with API key |
| `gpulab auth whoami` | Show current user |
| `gpulab ls` | List all containers |
| `gpulab inspect <uuid>` | Show container details |
| `gpulab deploy` | Deploy a new container |
| `gpulab stop <uuid>` | Stop a container |
| `gpulab start <uuid>` | Start a stopped container |
| `gpulab restart <uuid>` | Restart a container |
| `gpulab redeploy <uuid>` | Redeploy a container |
| `gpulab rm <uuid>` | Delete a container |
| `gpulab logs <uuid>` | View container logs |
| `gpulab stats <uuid>` | View resource usage |
| `gpulab exec <uuid> -- <cmd>` | Execute a command |
| `gpulab ssh <uuid>` | Interactive terminal |
| `gpulab templates` | List templates (Docker images) |
| `gpulab templates info <uuid\|name>` | Show template details |
| `gpulab templates categories` | List template categories |
| `gpulab templates create --name <n> --image <img>` | Create a template |
| `gpulab templates edit <uuid\|name> [flags]` | Edit a template |
| `gpulab templates delete <uuid\|name>` | Delete a template |
| `gpulab credentials` | List Docker registry credentials |
| `gpulab credentials add --username <u> --password <p>` | Store a registry credential |
| `gpulab credentials rm <id>` | Delete a registry credential |
| `gpulab gpus types` | List GPU types with free vs placeable counts |
| `gpulab gpus available` | Free GPUs by type, with placeable counts |
| `gpulab volumes` | List volumes |
| `gpulab volumes sync <dir> <volume> [remote]` | rsync-style sync a local dir to a volume |
| `gpulab volumes endpoints <volume>` | List endpoints that use a volume (reverse lookup) |
| `gpulab volumes files upload <volume> <file> --to <path>` | Upload a file, renamed |
| `gpulab serverless` | Manage serverless GPU endpoints |
| `gpulab serverless deploy <endpoint> <file>` | Upload → verify → rolling restart → smoke test |
| `gpulab serverless health <endpoint>` | Error rate, latency, queue, policy metrics, blocked scale-ups |
| `gpulab serverless list --with-policy` | List endpoints with their autoscaling policy loaded |
| `gpulab serverless trace <request-uuid>` | Trace a request's lifecycle |
| `gpulab serverless restart <endpoint> [replica\|--all]` | Restart replicas (rolling/drain) |
| `gpulab serverless replicas <endpoint> --stale` | Replica ages + stale-code detection |
| `gpulab serverless logs <endpoint>` | Replica logs (`--since`, `--grep`, `--detect`, `--replica all`) |
| `gpulab serverless requests <endpoint>` | View serverless request logs |
| `gpulab serverless autoscaling-logs <endpoint>` | View autoscaling history + summary |
| `gpulab update` | Update the CLI from GitHub Releases |

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format (for scripting/AI agents) |
| `-q, --quiet` | Quiet output (UUIDs only) |
| `--api-key` | Override API key |
| `--debug` | Debug mode (show HTTP requests) |
| `--tz` | Timestamp timezone: `utc` (default), `local`, or an IANA name |

## Configuration

Config is stored at `~/.gpulab/config.json`. API key priority:

1. `--api-key` flag
2. `GPULAB_API_KEY` environment variable
3. Config file

## AI Agent Usage

The CLI is designed for AI agent integration (Claude Code, Cursor, etc.):

```bash
export GPULAB_API_KEY=gpulab_xxx

# Deploy with env vars and capture UUID
UUID=$(gpulab deploy \
  --name ai-test \
  --template pytorch \
  --gpu-type "RTX 4090" \
  -e HF_TOKEN -e WANDB_API_KEY \
  --wait --json | jq -r '.container_id')

# Run commands
gpulab exec $UUID -- nvidia-smi
gpulab exec $UUID -- python -c "print('hello')"

# Write and run a script
gpulab exec $UUID -- sh -c 'echo "print(42)" > /workspace/test.py'
gpulab exec $UUID -- python /workspace/test.py

# JSON output for parsing
gpulab ls --json
gpulab stats $UUID --json

# Cleanup
gpulab stop $UUID
gpulab rm $UUID --force
```
