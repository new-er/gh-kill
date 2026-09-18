package main

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// killModel builds a program mid-kill-loop with all maps initialised, like the
// real program does through newProgram.
func killModel() program {
	return program{
		runs:    []Run{{ID: 1, Name: "ci", Status: "in_progress"}},
		killing: map[int]bool{0: true},
		sel:     map[int]bool{0: true},
		killed:  map[int]bool{},
		errs:    map[int]error{},
	}
}

// updateResults is the heart of the kill loop: it folds a poll result into the
// model and decides whether to keep polling or quit. It must stop exactly when
// every in-flight run reads "cancelled", and keep going while any don't.
func TestUpdateResults_Terminates(t *testing.T) {
	m := killModel()

	// One more poll: still in progress -> must keep polling, not quit.
	out, cmd := m.updateResults(resultsMsg{results: map[int]statusRes{0: {status: "in_progress"}}})
	m2 := out.(program)
	if cmd == nil {
		t.Fatal("expected a follow-up poll cmd while a run is still in_progress")
	}
	if !m2.killing[0] {
		t.Fatal("in_progress run should still be pending")
	}

	// Next poll: cancelled -> done, must quit (no follow-up poll cmd).
	out2, cmd2 := m2.updateResults(resultsMsg{results: map[int]statusRes{0: {status: "cancelled"}}})
	m3 := out2.(program)
	if cmd2 == nil {
		t.Fatal("expected a quit cmd once all runs are cancelled")
	}
	if !m3.killed[0] {
		t.Fatal("run should be marked killed")
	}
	if _, ok := m3.killing[0]; ok {
		t.Fatal("run should be removed from the pending set")
	}
}

func TestUpdateResults_StatusErrorKeepsPolling(t *testing.T) {
	m := killModel()
	out, cmd := m.updateResults(resultsMsg{results: map[int]statusRes{0: {err: errors.New("boom")}}})
	m2 := out.(program)
	// Errors are recorded but not terminal; the loop continues.
	if cmd == nil {
		t.Fatal("expected to keep polling after a status error")
	}
	if m2.errs[0] == nil {
		t.Fatal("expected the status error to be recorded")
	}
	if _, ok := m2.killing[0]; !ok {
		t.Fatal("run should remain pending on a status error")
	}
}

// Space toggles the focused row's selection. Regression: bubbletea renders the
// space key's KeyMsg.String() as " " (not "space"), so this guards the exact
// string the handler must match.
func TestSpaceTogglesSelection(t *testing.T) {
	m := newProgram("r", []Run{{ID: 1}, {ID: 2}})
	out, _ := m.handleKey(tea.KeyMsg{Type: tea.KeySpace})
	m2 := out.(program)
	if !m2.sel[0] {
		t.Fatal("space should select the focused row")
	}
	out2, _ := m2.handleKey(tea.KeyMsg{Type: tea.KeySpace})
	m3 := out2.(program)
	if m3.sel[0] {
		t.Fatal("a second space should deselect the row")
	}
}

// startKill must not emit anything (no poll) when nothing is selected.
func TestStartKill_NoSelection(t *testing.T) {
	m := newProgram("", []Run{{ID: 1}, {ID: 2}})
	m.mode = modeList
	// Nothing selected.
	out, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := out.(program)
	if m2.mode != modeList {
		t.Fatalf("expected to stay in list mode, got %v", m2.mode)
	}
}
