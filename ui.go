package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const pollInterval = time.Second

// prog is the running bubbletea program, set in main before the run loop,
// so background pollers can push messages back into it (Send is goroutine-safe).
var prog *tea.Program

type mode int

const (
	modeList mode = iota
	modeKill
)

type program struct {
	repo    string
	runs    []Run
	sel     map[int]bool
	focus   int
	mode    mode
	killing map[int]bool
	killed  map[int]bool
	errs    map[int]error
	note    string
}

type checkMsg struct{}

type statusRes struct {
	status string
	err    error
}

type resultsMsg struct {
	results map[int]statusRes
}

func newProgram(repo string, runs []Run) program {
	return program{
		repo:    repo,
		runs:    runs,
		sel:     map[int]bool{},
		focus:   0,
		killed:  map[int]bool{},
		killing: map[int]bool{},
		errs:    map[int]error{},
	}
}

func (m program) Init() tea.Cmd { return nil }

func (m program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case resultsMsg:
		return m.updateResults(msg)
	case checkMsg:
		return m.startCheck()
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		return m, nil
	}
	return m, nil
}

func (m program) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		if m.mode == modeKill {
			m.note = "aborted — cancels already issued are in progress"
		}
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	}

	if m.mode == modeKill {
		return m, nil
	}

	switch msg.String() {
	case "up", "k", "ctrl+p":
		if m.focus > 0 {
			m.focus--
		}
	case "down", "j", "ctrl+n":
		if m.focus < len(m.runs)-1 {
			m.focus++
		}
	case " ": // tea.KeyMsg.String() renders the space key as a literal " "
		m.sel[m.focus] = !m.sel[m.focus]
	case "a":
		for i := range m.runs {
			m.sel[i] = true
		}
		m.note = "all selected"
	case "enter":
		return m.startKill()
	}
	return m, nil
}

func (m program) startKill() (tea.Model, tea.Cmd) {
	var targets []int
	for i := range m.runs {
		if m.sel[i] {
			targets = append(targets, i)
		}
	}
	if len(targets) == 0 {
		m.note = "nothing selected"
		return m, nil
	}

	m.mode = modeKill
	m.killing = map[int]bool{}
	for i := range targets {
		m.killing[i] = true
		if err := cancel(m.repo, m.runs[i].ID); err != nil {
			m.errs[i] = err
			m.note = fmt.Sprintf("cancel %d: %v", m.runs[i].ID, err)
		}
	}
	m.note = fmt.Sprintf("cancelling %d run(s)…", len(targets))
	return m, m.cmdCheck()
}

// startCheck launches a goroutine that polls statuses and sends results back.
func (m program) startCheck() (tea.Model, tea.Cmd) {
	targets := make([]int, 0, len(m.killing))
	for i := range m.killing {
		targets = append(targets, i)
	}
	if len(targets) == 0 {
		m.finish()
		return m, tea.Quit
	}

	go func() {
		res := map[int]statusRes{}
		for _, i := range targets {
			st, err := statusOf(m.repo, m.runs[i].ID)
			res[i] = statusRes{status: st, err: err}
		}
		prog.Send(resultsMsg{results: res})
	}()
	return m, nil
}

func (m program) cmdCheck() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return checkMsg{} })
}

func (m program) updateResults(r resultsMsg) (tea.Model, tea.Cmd) {
	var note string
	for i, sr := range r.results {
		if sr.err != nil {
			m.errs[i] = sr.err
			note = fmt.Sprintf("status %d: %v", m.runs[i].ID, sr.err)
			continue
		}
		if sr.status == "cancelled" {
			delete(m.killing, i)
			m.killed[i] = true
		}
	}
	if len(m.killing) > 0 {
		m.note = fmt.Sprintf("%d run(s) still not cancelled…", len(m.killing))
		if note != "" {
			m.note = note
		}
		return m, m.cmdCheck()
	}
	m.finish()
	return m, tea.Quit
}

func (m program) finish() {
	done := len(m.killed)
	nerr := len(m.errs)
	switch {
	case nerr == 0:
		m.note = fmt.Sprintf("done — %d cancelled", done)
	default:
		m.note = fmt.Sprintf("done — %d cancelled, %d errored", done, nerr)
	}
}

func (m program) View() string {
	var b strings.Builder
	b.WriteString("gh-ci-kill\n")
	sub := fmt.Sprintf("%d active run(s) · %d selected", len(m.runs), len(m.sel))
	if m.mode == modeKill {
		sub = fmt.Sprintf("%d cancelling · %d done", len(m.killing), len(m.killed))
	}
	b.WriteString(sub)
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", 64))
	b.WriteByte('\n')

	for i, r := range m.runs {
		marker := " "
		if i == m.focus && m.mode == modeList {
			marker = ">"
		}
		box := " "
		if m.sel[i] {
			box = "x"
		}
		state := r.Status
		if m.killed[i] {
			state = "cancelled ✓"
		} else if m.killing[i] {
			state = "cancelling…"
		}
		b.WriteString(fmt.Sprintf("%s [%s] %-30s %-16s %s\n",
			marker, box, truncate(r.Name, 30), truncate(r.Branch, 16), state))
		if e, ok := m.errs[i]; ok {
			b.WriteString(fmt.Sprintf("      err: %v\n", e))
		}
	}

	b.WriteByte('\n')
	hint := "j/k or ↑/↓ move · space select · a all · enter kill · esc quit"
	if m.mode == modeKill {
		hint = "polling until cancelled · esc abort wait"
	}
	b.WriteString(hint)
	if m.note != "" {
		b.WriteString("   ·  " + m.note)
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
