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
  --gpu-type "RTX 4090" \
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
  --gpu-type "RTX 4090" \
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
| `gpulab gpus types` | List GPU types |
| `gpulab volumes` | List volumes |
| `gpulab serverless` | Manage serverless GPU endpoints |
| `gpulab serverless logs <endpoint>` | View serverless replica container logs |
| `gpulab serverless requests <endpoint>` | View serverless request logs |
| `gpulab serverless autoscaling-logs <endpoint>` | View autoscaling history |
| `gpulab update` | Update the CLI from GitHub Releases |

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format (for scripting/AI agents) |
| `-q, --quiet` | Quiet output (UUIDs only) |
| `--api-key` | Override API key |
| `--debug` | Debug mode (show HTTP requests) |

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
