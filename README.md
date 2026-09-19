# gh-kill 🔪

A tiny CLI that keeps killing stuck GitHub Actions runs until they're actually gone. Because sometimes GitHub Actions just refuses to die.

## Why it exists

`gh run cancel` all too often reports success while the run keeps running for minutes — you end up re-issuing the cancel over and over, checking the UI, cursing. `gh-kill` does that loop for you: it cancels, waits a second, checks the status, cancels again — until the run is genuinely `cancelled`, showing you a live count of how many kicks it took.

It also lists runs from the raw API, so PR-triggered runs show up — which `gh run list` skips by default.

## Install

Requires:

- [GitHub CLI](https://cli.github.com/) (`gh`), authenticated
- [Go](https://go.dev/) 1.25+
- `jq` (used to parse API responses)

From source:

```sh
git clone https://github.com/new-er/gh-kill
cd gh-kill
go build -o gh-kill .
```

## Usage

```sh
gh-kill                     # current git repo
gh-kill -r OWNER/REPO       # specific repo
gh-kill -l 50               # list at most 50 runs (default 100)
```

TUI keys:

| Key | Action |
|---|---|
| `j` / `k` or `↑` / `↓` | move |
| `space` | toggle selection |
| `a` | select all |
| `enter` | cancel selected runs (loops until done) |
| `esc` / `q` | quit |

## Contributing

Contributions welcome — just open an issue or a PR.

## License

MIT
