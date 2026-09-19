package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGhScript is a `gh` stand-in for tests. Crucially, its `api` handler runs
// real `jq` with the exact expression the app sent via -q against a payload, so
// the test exercises the actual query in gh.go (not a hand-canned array). That
// is what lets it catch the "empty stdout / NDJSON" bugs that a fake emitting a
// fixed array would hide.
const fakeGhScript = `#!/usr/bin/env bash
sd="${GH_FAKE_STATE:-/tmp}"
state="$sd/state"
cancellog="$sd/state.cancel"

payload="${GH_FAKE_PAYLOAD:-{\\"workflow_runs\\":[]}}"

if [ "$1" = "-R" ]; then shift 2; fi

case "$1" in
  api)
    expr="" prev=""
    for a in "$@"; do
      if [ "$prev" = "-q" ]; then expr="$a"; fi
      prev="$a"
    done
    jq -c "$expr" <<< "$payload"
    ;;
  run)
    case "$2" in
      cancel) echo "cancel $3" >> "$cancellog"; echo "Cancelled run" ;;
      view)
        c=$(cat "$state" 2>/dev/null || echo 0)
        c=$((c+1)); echo "$c" > "$state"
        if [ "$c" -lt 3 ]; then echo "in_progress"; else echo "cancelled"; fi
        ;;
    esac
    ;;
  repo) echo "new-er/gh-kill" ;;
esac
exit 0
`

// installFake puts the fake gh on PATH with the given state dir and optional
// api payload override. Returns the state dir.
func installFake(t *testing.T, payload string) string {
	dir := t.TempDir()
	t.Setenv("GH_FAKE_STATE", dir)
	fake := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fake, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "gh"), []byte(fakeGhScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	if payload != "" {
		t.Setenv("GH_FAKE_PAYLOAD", payload)
	}
	return dir
}

// TestGhRoundTrip drives the real child-process calls in gh.go (listActive,
// cancel, statusOf) and proves the poll loop reaches "cancelled" even when a
// run reads "in_progress" for its first couple of polls.
func TestGhRoundTrip(t *testing.T) {
	dir := installFake(t, `{"workflow_runs":[
		{"id":101,"name":"build + test","head_branch":"main","status":"in_progress"},
		{"id":102,"name":"e2e","head_branch":"release/1.2","status":"queued"}
	]}`)
	repo := "new-er/gh-kill"

	runs, err := listActive(repo, 100)
	if err != nil {
		t.Fatalf("listActive: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 active runs, got %d: %+v", len(runs), runs)
	}
	if runs[0].ID != 101 || runs[0].Status != "in_progress" {
		t.Fatalf("unexpected first run: %+v", runs[0])
	}

	if err := cancel(repo, 101); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	cancellog, _ := os.ReadFile(filepath.Join(dir, "state.cancel"))
	if !strings.Contains(string(cancellog), "cancel 101") {
		t.Fatalf("cancel was not issued for run 101; log=%q", string(cancellog))
	}

	// Simulate the kill loop: keep polling statusOf until it reads "cancelled".
	// Bounded so a regression that never flips would fail rather than hang.
	var got string
	for i := 0; i < 5; i++ {
		s, err := statusOf(repo, 101)
		if err != nil {
			t.Fatalf("statusOf: %v", err)
		}
		got = s
		if s == "cancelled" {
			break
		}
	}
	if got != "cancelled" {
		t.Fatalf("poll loop never reached 'cancelled'; last=%q", got)
	}
}

// TestListActive_ZeroActive is the regression for the reported
// "unexpected end of JSON input": with no non-completed runs the API reply,
// filtered by jq, is the empty set — the app must still parse it as zero runs
// instead of failing json.Unmarshal on empty/NDJSON output.
func TestListActive_ZeroActive(t *testing.T) {
	payload := `{"workflow_runs":[
		{"id":201,"name":"old build","head_branch":"main","status":"completed"},
		{"id":202,"name":"old e2e","head_branch":"main","status":"completed"}
	]}`
	installFake(t, payload)

	runs, err := listActive("new-er/gh-kill", 100)
	if err != nil {
		t.Fatalf("listActive with no active runs: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected 0 active runs, got %d: %+v", len(runs), runs)
	}
}
