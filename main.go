package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	repo := flag.String("r", "", "repo OWNER/REPO (empty = current git context)")
	limit := flag.Int("l", 100, "max runs to list")
	flag.Parse()

	runs, err := listActive(*repo, *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(runs) == 0 {
		fmt.Println("no active runs")
		return
	}

	prog = tea.NewProgram(newProgram(*repo, runs))
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
