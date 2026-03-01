# GPULab CLI

Command-line interface for [GPULab](https://gpulab.ai) — deploy, manage, and interact with GPU containers from your terminal.

## Install

```bash
# macOS / Linux
curl -fsSL https://gpulab.ai/cli.sh | sh

# Or build from source
go install github.com/gpulab/gpulab-cli/cmd/gpulab@latest

# Or clone and build
git clone https://github.com/gpulab/gpulab-cli.git
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
| `gpulab templates` | List templates |
| `gpulab gpus types` | List GPU types |
| `gpulab volumes` | List volumes |

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
