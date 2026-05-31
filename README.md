# GTR — Go Test Runner

[![Go Report Card](https://goreportcard.com/badge/github.com/mmirolim/gtr)](https://goreportcard.com/badge/github.com/mmirolim/gtr)

**Smart test selection for Go.** GTR uses call-graph analysis (CHA/VTA/RTA/static) and coverage profiling to identify which tests are affected by your code changes, then runs *only* those tests. Works with any Go project using the standard `testing` package.

## Features

| Feature | Description |
|---|---|
| **Call-graph analysis** | VTA, CHA, RTA, static — picks the right tests by tracing the call graph |
| **Coverage strategy** | Uses `go test -coverprofile` data to map code ↔ tests |
| **Watch mode** | Auto-runs affected tests on file save with desktop notifications |
| **MCP server** | JSON-RPC 2.0 over stdio — integrates with AI coding assistants |
| **One-shot mode** | `gtr run` for CI; `gtr analyze` for JSON output |
| **SSA caching** | Caches the SSA call graph, invalidated by file content hash |
| **Structured logging** | `log/slog` throughout for machine-parseable output |
| **Auto-commit** | Optionally commit on green tests |

## Installation

Requires **Go 1.24+** and **git**.

```bash
go install github.com/mmirolim/gtr@latest
```

## Quick Start

```bash
# Watch mode (default) — auto-test on file changes
gtr

# One-shot: find affected tests and run them
gtr run

# One-shot: print affected tests as JSON (for CI)
gtr analyze

# Start MCP server for AI assistant integration
gtr mcp
```

## Commands

| Command | Description |
|---|---|
| `gtr watch` | Watch for file changes, run affected tests (default) |
| `gtr run` | Analyze changes, run affected tests, exit |
| `gtr analyze` | Analyze changes, print affected tests as JSON |
| `gtr mcp` | Start MCP JSON-RPC 2.0 server over stdio |
| `gtr help` | Show help |

## Flags

```
-C string          directory to watch (default ".")
-strategy string   strategy: analysis or coverage (default "analysis")
-analysis string   analysis algorithm: vta, cha, rta, static (default "vta")
-run-init bool     run init steps on first run (default true)
-args string       args to the test binary
-auto-commit bool  auto commit on tests pass (default false)
-delay int         delay in Milliseconds (default 1000)
-exclude-dirs      prefixes to exclude sep by comma (default "vendor,node_modules")
-exclude-file-prefix  prefixes to exclude sep by comma (default "#")
-base-ref string   git ref to diff against (e.g. main, HEAD~1)
```

## Strategies

### Analysis (default)

Uses SSA-based call-graph analysis from `golang.org/x/tools`. Given a code change, GTR traces which functions are affected, walks the call graph to find test functions that transitively call those functions, and runs only those tests.

Available algorithms: `vta` (default, most precise), `cha`, `rta`, `static`.

```bash
gtr -strategy=analysis -analysis=vta
```

### Coverage

Uses `go test -coverprofile` data. On first run, GTR runs all tests to collect coverage profiles (stored in `.gtr/`). On subsequent runs, it maps changed lines to covered tests.

```bash
gtr -strategy=coverage
```

## MCP Integration

GTR exposes an [MCP](https://modelcontextprotocol.io/) server for AI coding assistants.

### Configuration

Add to your MCP client configuration (e.g. `~/.gemini/settings.json`):

```json
{
  "mcpServers": {
    "gtr": {
      "command": "gtr",
      "args": ["mcp"]
    }
  }
}
```

### Tools

| Tool | Description |
|---|---|
| `affected_tests` | Find tests affected by code changes |
| `run_affected_tests` | Find and run affected tests |
| `file_impact_analysis` | Full impact report: changed files, functions, affected tests |

### Example

```jsonc
// Request
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"affected_tests","arguments":{"work_dir":".","analysis":"vta"}}}

// Response
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"tests\":[\"pkg.TestFoo\"],\"subtests\":[],\"run_all\":false}"}]}}
```

## Edge Cases

- Works with Go standard `testing` library only
- Reflection-based calls may not be resolved (coverage strategy helps)
- Large projects may be slow on first analysis (SSA cache speeds up subsequent runs)
- May fail with "too many open files" — increase ulimit if needed

## How It Works

1. **Detect changes** via `git diff` (uncommitted changes or against a base ref)
2. **Parse affected code blocks** using Go AST
3. **Build call graph** (SSA analysis) or look up coverage profiles
4. **Identify tests** that transitively call changed functions
5. **Run only those tests** with `go test -run <pattern>`

## License

MIT
