# boiler CLI

Admin control-plane CLI for Boiler — drive the whole instance from a terminal (built for
admin **agents** as well as humans). Databases, tables, rows/SQL, endpoints, webhooks,
embeddings, tokens, local inference.

It's a thin, auditable client over Boiler's existing `/admin` API. Every action is a visible,
logged command.

## Install

**One-liner (no Go needed)** — downloads a prebuilt binary to your PATH:
```sh
curl -fsSL https://raw.githubusercontent.com/untoldecay/boiler-cli/main/install.sh | sh
# pin a version:
curl -fsSL https://raw.githubusercontent.com/untoldecay/boiler-cli/main/install.sh | sh -s -- v0.2.0
```

**With Go:**
```sh
go install github.com/untoldecay/boiler-cli/cmd/boiler@latest
```
Installs `boiler` into `$(go env GOPATH)/bin` — make sure that's on your PATH
(`export PATH="$PATH:$(go env GOPATH)/bin"`).

## Build from source
```sh
git clone https://github.com/untoldecay/boiler-cli && cd boiler-cli
go build -o boiler ./cmd/boiler
# cross-compile, e.g. linux: GOOS=linux GOARCH=amd64 go build -o boiler ./cmd/boiler
```

## Auth
```sh
boiler login --server https://your-boiler.example.com    # prompts email + password (hidden)
```
Stores a **short-lived** JWT in `~/.boiler/config.json` and auto-refreshes it. For headless/CI use
`BOILER_SERVER` + `BOILER_TOKEN` env vars instead.

```sh
boiler whoami      # current session
boiler status      # instance version/health
boiler logout
```

## Human-in-the-loop safety
- Read commands run freely.
- **Critical/destructive actions require `--confirm`** — `db delete`, `webhook create` (outbound),
  public endpoints, `tokens create/delete`, and any non-SELECT `sql`. Without it the CLI refuses and
  explains. (When an agent runs `boiler`, the human still approves the shell command in the client,
  so `--confirm` makes the destructive intent explicit in what gets approved.)

## Commands (overview)
```sh
# data
boiler db list | create --name X --confirm | delete --name X --confirm
boiler tables list --db X
boiler tables describe --db X --table T
boiler rows fetch --db X --table T [--limit 50 --offset 0]
boiler sql --db X "SELECT …"                 # writes need --confirm

# API endpoints (friendly flags, or --json for anything)
boiler endpoints list --db X [--table T]
boiler endpoints create --db X --name "digest read" --table daily_digest --columns id,title \
  [--filterable tag --sort id:desc --access protected --token <id>]
boiler endpoints create --db X --name "digest search" --table daily_digest --type vector \
  --columns id,title --vector-column daily_digest_embedding --provider ollama --model nomic-embed-text
boiler endpoints delete --id … --confirm

# webhooks
boiler webhooks list --db X [--table T]
boiler webhooks create --db X --table T --name n8n --url https://… --events insert,update --confirm
boiler webhooks test --id …
boiler webhooks delete --id … --confirm

# embeddings
boiler embed providers
boiler embed run --db X --table T --template '{{summary}}' --provider ollama --model nomic-embed-text [--column T_embedding]
boiler embed key set --provider openai --key sk-… --confirm      # cloud provider keys
boiler embed key remove --provider openai
boiler embed auto list --db X                                     # keep-in-sync configs
boiler embed auto delete --id …

# tokens
boiler tokens list
boiler tokens create --name t --db X --perms read,write --confirm
boiler tokens reveal --id … --confirm
boiler tokens delete --id … --confirm

# users
boiler users list
boiler users create --email a@b.com --password … --role admin --confirm
boiler users delete --id … --confirm

# local inference
boiler infra local-inference status
boiler infra local-inference pair
```

## Notes
- Permissions are enforced server-side (RBAC) regardless of the CLI — an admin token gets admin scope.
- `sql` runs through `/admin/execute`, which classifies read/write/admin per the same RBAC as the UI.
