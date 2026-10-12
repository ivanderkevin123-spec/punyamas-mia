# Audit: Environment

**Project:** Punyamas Marketing Intelligence Agent (MIA) · **Folder:** `~/Documents/punyamas/mia`
**Audit date:** 2026-10-12 (WIB) · **Method:** read-only shell commands from a sandboxed Claude Code session. Nothing was installed, started, or changed. Credential values were never printed.

Legend: **[V]** verified by direct inspection · **[A]** assumption · **[?]** unresolved question.

## 1. Verified facts

### 1.1 Machine and operating system

| Item | Finding |
|---|---|
| Host | `Kevins-MacBook-Air.local` |
| OS | macOS 26.6.2 (build 25G83), Darwin 25.6.0 |
| Architecture | arm64 (Apple Silicon; kernel tag `T8103` indicates the M1 family) |
| Disk | 460 GiB volume, 217 GiB available (50 % used) |
| Timezone | WIB (UTC+7) |
| Compiler | Xcode command-line tools present (`/Applications/Xcode.app/Contents/Developer`), Apple clang 21 → cgo is possible |

### 1.2 Programming languages and runtimes

| Language | Location | Version | Architecture | Notes |
|---|---|---|---|---|
| Go (on PATH) | `/usr/local/go/bin/go` | **1.17.5** | x86_64 (runs under Rosetta) | Too old for current libraries; `GOTOOLCHAIN` auto-download is not supported by 1.17 |
| Go (native) | `~/sdk/go1.27.2/bin/go` | **1.27.2** | arm64 | Not on PATH. Used by `~/Documents/kerjalancar` via `GO=~/sdk/go1.27.2/bin/go make …`. `GOTOOLCHAIN=auto`, `GOMODCACHE=~/go/pkg/mod`, `GOCACHE=~/Library/Caches/go-build` |
| Python (on PATH) | `/usr/local/bin/python3` | 3.9.10 | x86_64 (Homebrew, Intel prefix) | Python 3.9 reached end of life in October 2025 |
| Python (system) | `/usr/bin/python3` | 3.9.6 | universal | Apple-provided |
| Python 3.12 | `/usr/local/opt/python@3.12/bin/python3.12` | 3.12.14 | x86_64 | Installed via Homebrew formula `python@3.12`, not linked as `python3` |
| Node (nvm default) | `~/.nvm/versions/node/v10.24.1` | **10.24.1** | — | nvm default alias is `10`; too old for any current tooling |
| Node 20 / 22 | `~/.nvm/versions/node/v20.20.2`, `v22.23.2` | 20.20.2, 22.23.2 | — | `npm` under v22 only works after `nvm use 22` (otherwise the v10 binary is picked up). `pnpm`, `yarn` present under v22. `node:sqlite` loads on v22 with an `ExperimentalWarning` |
| Ruby | `~/.rvm/rubies/ruby-2.6.5` | 2.6.5 | x86_64 | Not relevant |
| Java | `/usr/bin/java` stub | — | — | No runtime installed ("Unable to locate a Java Runtime") |
| Rust | — | — | — | Not installed |

### 1.3 Package managers and build tools

| Tool | Status |
|---|---|
| Homebrew | 6.0.5 at `/usr/local` (Intel prefix under Rosetta); no `/opt/homebrew` |
| nvm, rvm | Present |
| pip 21.3.1 (for Python 3.9) | Installed packages are minimal: pytest 8.4.2, PyYAML 6.0.3, jsonschema 4.25.1, click 8.1.8 and transitive deps (22 total). No `uv`, `poetry`, `pipx` |
| make | GNU Make 3.81 |
| git | 2.54.0 (Apple Git-157). `gh` CLI not installed |
| jq, curl | Present. `wget`, `tmux`, `cmake` absent |
| Docker | CLI 20.10.11 present; no `dockerd` process found; daemon socket unreachable from the sandbox |

### 1.4 Go module caches (relevant to dependency approval)

| Cache | Contents relevant to MIA |
|---|---|
| `~/go/pkg/mod` (587 MB) | 2022-era modules from the `forstok_project` services: go-chi, gorilla, gorm, go-sql-driver/mysql, testify, gopkg.in/yaml.v2, joho/godotenv, urfave/cli. No SQLite driver cached except a download record for `mattn/go-sqlite3` (cgo) |
| `~/Documents/kerjalancar/.toolchain/modcache` | **modernc.org/sqlite v1.60.1** (pure Go), `go.yaml.in/yaml/v3`, go-chi/chi v5, jackc/pgx v5, stretchr/testify, pressly/goose |

Outbound access to `proxy.golang.org` was verified (HTTP 200), so modules not cached can be fetched once dependency approval is given.

### 1.5 Database services

| Service | Installed | Running | Notes |
|---|---|---|---|
| SQLite | `/usr/bin/sqlite3` 3.51.0 | n/a (library) | Sufficient for MVP storage |
| MySQL 8.0.28 | Yes (Homebrew) | **No** (`brew services` = none; socket connect fails) | Not needed |
| Redis 6.2.6 | Yes (Homebrew) | **Yes**, 127.0.0.1:6379 (brew service started) | Not needed; connection from the sandbox is blocked, listening socket verified via `lsof` |
| RabbitMQ | Yes | Stopped | Not needed |
| PostgreSQL | No native install | — | `kerjalancar` uses Docker Compose for Postgres |
| MongoDB | No | — | — |

### 1.6 Scheduling facilities

| Facility | Finding |
|---|---|
| launchd user agents | `~/Library/LaunchAgents` contains the user's own `com.kevinivander.xauea.{detect,flask,review}.plist` (StartInterval 900 s, RunAtLoad, stdout/stderr to the project's `logs/`), plus Homebrew (`redis`, `rabbitmq`), Google, Grammarly and VirtualBox agents. This is a proven, user-owned scheduling pattern on this machine |
| cron | `crontab -l` is blocked inside the sandbox; state unknown |
| Claude Code scheduled tasks / routines | Available to this session, but they run Claude, not the application |

### 1.7 Network and web research tooling (from the sandbox)

| Host | Result |
|---|---|
| https://proxy.golang.org | 200 |
| https://pypi.org | 200 |
| https://registry.npmjs.org | 200 |
| https://api.anthropic.com | reachable (404 on `HEAD /`, as expected) |
| https://trends.google.com | 301 |
| https://www.google.com | 200 |

Outbound HTTP works for hosts declared to the sandbox proxy. No headless browser is installed (no Playwright cache); `~/Documents/tradingview-mcp` uses `chrome-remote-interface` (Chrome DevTools Protocol), which requires a running Chrome.

### 1.8 AI provider configuration (names only; values not read)

| Item | Finding |
|---|---|
| Environment variables matching key/token/api | `ANTHROPIC_BASE_URL`, `API_TIMEOUT_MS`, `CLAUDE_CODE_MESSAGING_TOKEN`, `BAGGAGE` — all set by the Claude Code session harness |
| `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / search-provider keys | **Not present** |
| `~/.config/anthropic/` | Exists, **empty** |
| Claude Code CLI | `~/.local/bin/claude`, version 2.1.226; supports `-p/--print` and `--output-format json|stream-json` (headless use). Authenticated via the user's subscription login |
| Local LLM runtimes (Ollama, LM Studio, llama.cpp) | Not installed |
| MCP servers in `~/.claude.json` | None at user level; several projects have empty project-level entries |
| Session-only tools | This Claude Code session has WebSearch/WebFetch and MCP connectors (Google Drive, Canva, Vercel, Alpha Vantage, FMP). These are **not** available to an application process |

### 1.9 Git

| Item | Finding |
|---|---|
| `~/Documents/punyamas/mia` | **Not a git repository** (only `.claude/` exists) |
| Global git config | Keys: `core.excludesfile`, Sourcetree diff/merge tools. **No global `user.name`/`user.email`**; sibling repos set identity locally (commits in `kerjalancar` are by "Kevin Ivander") |

### 1.10 Ports currently in use on 127.0.0.1

| Port | Process | Owner |
|---|---|---|
| 5000 | Python (Flask) | `xau-ea-bot` launchd agent |
| 6379 | redis-server | Homebrew service |
| 8791 | node | `bots/punyamas` `server-tiruan` (running at audit time) |
| 5037, 5554, 5555, 8554 | adb / qemu | Android emulator |
| 8765, 8767, 8781–8784 | reserved | `bots/.claude/launch.json` preview servers |

### 1.11 Test tools

`go test` (stdlib), pytest 8.4.2, vitest (inside `bots/punyamas`). `golangci-lint` not installed (`kerjalancar` falls back to `go vet`).

## 2. Assumptions

- **[A]** The CPU model is M1-class and RAM is 8–16 GB; `sysctl` was blocked by the sandbox so neither was measured. No MIA component needs more than a few hundred MB.
- **[A]** Docker Desktop is not running (no `dockerd`/`com.docker` process found). MIA does not need Docker.
- **[A]** Go 1.27.2 at `~/sdk` was installed with `golang.org/dl` and may be upgraded by the user at any time; MIA will reference it through a Makefile variable rather than a hard-coded path in code.
- **[A]** The sandbox network policy approximates what the application will see when run by the user outside the sandbox (i.e., the user's normal network has no additional egress restrictions).

## 3. Unresolved questions

- **[?]** CPU/RAM figures (cosmetic; not blocking).
- **[?]** Whether the user wants Go 1.27 added to `PATH` or kept as a Makefile variable (plan recommends the variable).
- **[?]** Whether `cron` is in use (not inspectable from the sandbox; launchd is the recommended facility anyway).
- **[?]** Which AI provider path will be authorized (API key with billing, headless `claude -p` under the subscription subject to terms verification, or mock only).

## 4. Implications for architecture

1. Use the native Go 1.27.2 toolchain via `GO ?= $(HOME)/sdk/go1.27.2/bin/go` in the Makefile (same pattern as `kerjalancar`).
2. Use a pure-Go SQLite driver (`modernc.org/sqlite`) to avoid cgo and keep a single static binary that launchd can run without environment setup.
3. Mock provider first; provider adapters are added only after credentials and terms are confirmed.
4. Avoid binding ports in the MVP; if a local viewer is ever added, bind 127.0.0.1 on a port outside the ranges listed in §1.10.
