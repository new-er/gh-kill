package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Run struct {
	ID     int64
	Name   string
	Branch string
	Status string
}

var activeStatus = `select(.status != "completed")`

// listActive returns all non-completed workflow runs for repo ("" = default repo).
// Uses the raw API endpoint because `gh run list` skips PR runs by default.
func listActive(repo string, limit int) ([]Run, error) {
	if repo == "" {
		out, err := ghJSON(repo, "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner")
		if err != nil {
			return nil, err
		}
		repo = strings.TrimSpace(string(out))
	}
	// Wrap the pipeline in [ ] so jq emits a JSON array (empty, not blank,
	// when no runs match) — json.Unmarshal rejects empty output and NDJSON.
	q := "[ .workflow_runs[] | " + activeStatus + " | {id: .id, name: .name, branch: .head_branch, status: .status} ]"
	out, err := ghJSON(repo, "api", "repos/"+repo+"/actions/runs?per_page="+itoa(limit), "-q", q)
	if err != nil {
		return nil, err
	}
	var runs []Run
	if err := json.Unmarshal(out, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// statusOf returns a run's current status.
func statusOf(repo string, id int64) (string, error) {
	out, err := ghOut(repo, "run", "view", itoa(id), "--json", "status", "-q", ".status")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// cancel issues `gh run cancel` for a run.
func cancel(repo string, id int64) error {
	_, err := ghOut(repo, "run", "cancel", itoa(id))
	return err
}

func ghOut(repo string, args ...string) ([]byte, error) {
	full := args
	// `gh api` takes the repo in the endpoint URL, not via -R (it rejects -R).
	if repo != "" && args[0] != "api" {
		full = append([]string{"-R", repo}, args...)
	}
	cmd := exec.Command("gh", full...)
	out, err := cmd.Output()
	if err != nil {
		var msg string
		if ee, ok := err.(*exec.ExitError); ok {
			msg = strings.TrimSpace(string(ee.Stderr))
		} else {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

func ghJSON(repo string, args ...string) ([]byte, error) { return ghOut(repo, args...) }

func itoa(n any) string { return fmt.Sprint(n) }
