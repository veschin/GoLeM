<p align="center">
  <img src="GoLeM.png" width="600" alt="GoLeM — a tiny wizard commanding clay golems to do the heavy lifting" />
</p>

<h1 align="center">GoLeM</h1>

<p align="center">
  <strong>One wizard. Unlimited golems. Zero Anthropic API costs.</strong>
</p>

<p align="center">
  Spawn autonomous Claude Code agents powered by GLM-5 via Z.AI.<br>
  Each golem is a full Claude Code instance — reads files, edits code, runs tests, uses MCP servers and skills.<br>
  You stay on Opus. Your golems run free and parallel through Z.AI. Ship faster.
</p>

---

![Architecture](docs/architecture.svg?v=4)

## install

Requirements: [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code), a [Z.AI Coding Plan](https://z.ai/subscribe) key, Go 1.25+.

```bash
go install github.com/veschin/GoLeM/cmd/glm@latest
glm _install
```

Or from source:

```bash
git clone https://github.com/veschin/GoLeM.git
cd GoLeM
go build -o glm ./cmd/glm/
./glm _install
```

`_install` does the following: creates `~/.config/GoLeM/`, prompts for your Z.AI API key (stored in `zai_api_key` with 0600 permissions), writes `glm.toml` with `permission_mode`, creates `~/.claude/subagents/`, injects delegation instructions into `~/.claude/CLAUDE.md` between `<!-- GLM-SUBAGENT-START -->` and `<!-- GLM-SUBAGENT-END -->` markers, and for source installs creates a symlink at `~/.local/bin/glm`.

If you already had a key in `~/.config/zai/env` (old format), it migrates automatically.

Shell completions are installed separately:

- **Bash:** `~/.local/share/bash-completion/completions/glm`
- **Fish:** `~/.config/fish/completions/glm.fish`
- **Zsh:** `~/.config/zsh/completions/_glm` (add `fpath=(~/.config/zsh/completions $fpath)` to `.zshrc`)

Restart your shell after install.

## update and uninstall

```bash
glm update          # git pull + go build for source installs, go install for go-install
glm _uninstall      # removes symlink, CLAUDE.md section, prompts about keys and job artifacts
```

## commands

```
glm session [flags] [claude flags]     # interactive claude code (syscall.Exec, replaces the process)
glm run   [flags] "prompt"             # synchronous: creates job, runs claude, prints stdout, removes job dir
glm start [flags] "prompt"             # asynchronous: same, but in a goroutine — prints job ID immediately
glm chain [flags] "p1" "p2" ...        # sequential: stdout of step N is injected into the prompt of step N+1
glm status  JOB_ID                     # reads the status file
glm result  JOB_ID                     # reads stdout.txt
glm log     JOB_ID                     # reads changelog.txt
glm list    [--status S] [--since D]   # lists job dirs with filters
glm clean   [--days N]                 # removes old jobs
glm kill    JOB_ID                     # SIGTERM → SIGKILL after one second
glm doctor                             # 6 checks: claude CLI, API key, Z.AI reachability, models, slots, platform
glm config  {show|set KEY VAL}         # show prints all values with source; set writes to glm.toml
glm update                             # updates glm
glm version                            # prints version
```

Examples:

```bash
glm run -d ~/project "find bugs in auth.go"
glm run -m glm-4 "refactor auth"               # all three slots → glm-4
glm run --opus glm-5 --haiku glm-4 "task"    # per-slot models
glm session --sonnet glm-4                      # interactive session with custom sonnet
glm run --unsafe "deploy hotfix"                # bypass permission checks
glm start -t 600 "long task"                    # async with 10-minute timeout
glm list --status running
glm list --status done,failed --since 2h
glm list --json                                 # JSON for scripting
glm chain "write tests for auth.go" "run tests and fix failures"
glm chain --continue-on-error "step1" "step2" "step3"
```

## flags

Apply to `session`, `run`, `start`, and `chain`.

| Flag | What it does |
|---|---|
| `-m`, `--model MODEL` | All three slots (opus, sonnet, haiku) → MODEL |
| `--opus MODEL` | Opus slot only |
| `--sonnet MODEL` | Sonnet slot only |
| `--haiku MODEL` | Haiku slot only |
| `-d DIR` | Working directory |
| `-t SEC` | Timeout in seconds (ignored for session) |
| `--unsafe` | bypassPermissions |
| `--mode MODE` | Permission mode: `bypassPermissions`, `acceptEdits`, `default`, `plan` |
| `--json` | JSON output for list, status, result, log |

Claude Code uses three model slots internally: heavy tasks go to opus, normal tasks to sonnet, fast tasks to haiku. All three default to `glm-5`. `-m` changes all at once; `--opus`/`--sonnet`/`--haiku` change them individually.

`session` passes unknown flags directly to `claude` — for example `--resume`, `--verbose`.

`chain` also accepts `--continue-on-error`: without it, the chain stops on the first failed step.

## configuration

`~/.config/GoLeM/glm.toml` is read on every `glm` invocation. Priority: CLI flag > environment variable > glm.toml > hardcoded default.

```bash
glm config show                   # all values with source labels: (default), (config), (env)
glm config set max_parallel 5
glm config set model glm-4
```

| Key | Env | Default | Description |
|---|---|---|---|
| `model` | `GLM_MODEL` | `glm-5` | Default model for all three slots |
| `opus_model` | `GLM_OPUS_MODEL` | (model) | Model for heavy tasks |
| `sonnet_model` | `GLM_SONNET_MODEL` | (model) | Model for normal tasks |
| `haiku_model` | `GLM_HAIKU_MODEL` | (model) | Model for fast tasks |
| `permission_mode` | `GLM_PERMISSION_MODE` | `bypassPermissions` | Default permission mode |
| `max_parallel` | `GLM_MAX_PARALLEL` | `3` | Max parallel agents |

Debug logging (`GLM_DEBUG=1`) is read directly from the environment, not from the config file:

```bash
GLM_DEBUG=1 glm run "task"                    # debug output to stderr
GLM_DEBUG=1 GLM_LOG_FORMAT=json glm doctor    # structured JSON logs
GLM_LOG_FILE=/tmp/glm.log glm run "task"      # also write logs to file
```

Log levels: `[D]` debug, `[+]` info, `[!]` warn, `[x]` error. Colors on TTY, plain text when piped.

### multi-provider

`glm.toml` supports `[providers.NAME]` sections — you can point glm at any OpenAI-compatible endpoint instead of Z.AI:

```toml
default_provider = "custom"

[providers.custom]
base_url = "https://custom.api.com/v1/anthropic"
api_key_file = "~/.config/GoLeM/custom_key"
timeout_ms = "5000"
opus_model = "custom-opus"
sonnet_model = "custom-sonnet"
haiku_model = "custom-haiku"
```

If no `[providers.*]` sections are defined, the hardcoded Z.AI defaults are used (`https://api.z.ai/api/anthropic`, `~/.config/GoLeM/zai_api_key`).

## how it works

**session** is `syscall.Exec`. `glm session` builds `argv` and the environment, then replaces itself with the `claude` process. No job directories, no output capture. The following are injected into the environment: `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `API_TIMEOUT_MS`, and the three model slots (`ANTHROPIC_DEFAULT_OPUS_MODEL` etc.).

**run** is synchronous. It creates a job dir under `~/.claude/subagents/<project-id>/<job-id>/`, writes metadata files (prompt.txt, workdir.txt, model.txt, etc.), runs `claude -p --no-session-persistence --output-format json` with a timeout context, captures stdout to `raw.json` and stderr to `stderr.txt`. On completion it parses `raw.json` — extracts `.result` into `stdout.txt` and generates `changelog.txt` from tool_use blocks. Then prints `stdout.txt` and removes the job dir.

**start** does the same, but in a goroutine. The job ID is printed immediately; the process then waits for SIGINT/SIGTERM.

**chain** runs steps sequentially. The prompt for step N+1 receives the result of step N as `"Previous agent result:\n{stdout}\n\nYour task:\n{prompt}"`. Each step's job dir is kept.

The project ID is derived from the absolute path of the working directory: `{basename}-{crc32(path)}`. This groups jobs by project without collisions between projects that share the same directory name.

**audit** — each job writes a changelog from tool_use events: Edit, Write, Bash (delete commands only), NotebookEdit. The full tool call history is in `raw.json`.

```bash
glm log job-20260226-143022-a1b2c3d4
# EDIT src/auth.py: 142 chars
# WRITE tests/test_auth.py
# DELETE via bash: rm tmp/cache.db
```

If an agent hits a permission wall, the status becomes `permission_error`, not just `failed`.

## testing

```bash
go test ./...                                     # unit tests, no API calls
go test -tags e2e ./internal/e2e/... -v           # e2e with a real claude and API key
```

E2E tests are tagged `//go:build e2e` and require a working `claude` CLI and a valid key.

## exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | User error (bad arguments, invalid config) |
| 3 | Not found (job does not exist) |
| 124 | Timeout |
| 127 | Dependency missing (claude CLI not found) |

Errors are written to stderr as `err:<category> "message"` for programmatic parsing.

## files

**Runtime:**

| Path | What |
|---|---|
| `~/.local/bin/glm` | Binary or symlink (source installs) |
| `~/.claude/CLAUDE.md` | Delegation instructions (between markers) |
| `~/.config/GoLeM/glm.toml` | Config: models, permissions, parallelism |
| `~/.config/GoLeM/zai_api_key` | Z.AI API key (chmod 600) |
| `~/.config/GoLeM/config.json` | Install metadata (version, install_mode) |
| `~/.claude/subagents/<project>/<job-id>/` | Job artifacts |

Inside a job dir: `prompt.txt`, `workdir.txt`, `model.txt`, `permission_mode.txt`, `started_at.txt`, `finished_at.txt`, `raw.json`, `stdout.txt`, `stderr.txt`, `changelog.txt`, `status`, `pid.txt`, `exit_code.txt` (on error only).

**Source (Go):**

| Path | What |
|---|---|
| `cmd/glm/main.go` | Entry point, CLI dispatch, signal handling |
| `internal/config/config.go` | TOML loading, env overrides, validation |
| `internal/config/provider.go` | Multi-provider: LoadProvider, ListProviders, ResolveModelEnv |
| `internal/cmd/session.go` | SessionCmd: builds argv+env for syscall.Exec |
| `internal/cmd/chain.go` | ChainCmd: sequential steps with stdout injection |
| `internal/cmd/flags.go` | ParseFlags, Validate |
| `internal/cmd/doctor.go` | DoctorCmd: 6 diagnostic checks |
| `internal/cmd/install.go` | InstallCmd, UninstallCmd, UpdateCmd, InjectClaudeMD |
| `internal/claude/claude.go` | Execute: subprocess, env, output capture |
| `internal/claude/parser.go` | ParseRawJSON, GenerateChangelog |
| `internal/job/job.go` | Job lifecycle, status machine, FindJobDir |
| `internal/slot/slot.go` | Concurrency control: flock/mkdir, PID liveness |
| `internal/log/log.go` | Structured logger (human + JSON) |
| `internal/exitcode/exitcode.go` | Typed errors, exit codes |
| `internal/e2e/` | End-to-end tests (//go:build e2e) |

## platforms

Linux (amd64, arm64), macOS (amd64, arm64), WSL.

## troubleshooting

```bash
glm doctor          # claude_cli, api_key, zai_reachable, models, slots, platform
```

| Symptom | Fix |
|---|---|
| `claude CLI not found` | Install Claude Code and add it to PATH |
| `credentials not found` | Run `glm _install` |
| Empty output after run | Check `glm result JOB_ID` or `~/.claude/subagents/.../stderr.txt` |
| `~/.local/bin` not in PATH | Add `export PATH="$HOME/.local/bin:$PATH"` to `.bashrc`/`.zshrc` |
| Jobs stuck in queued | Run `glm doctor` to inspect slots; clear stale jobs with `glm clean --days 0` |
| Status `permission_error` | Add `--unsafe` or set `permission_mode` to `bypassPermissions` |
