package fsutil

import "os/exec"

// Program is one "Open with..." candidate.
type Program struct {
	Label   string
	Command string
}

// candidateCommands mirrors the original app's hardcoded candidate list,
// each label paired with the binary to look up on PATH.
var candidateCommands = []struct{ label, command string }{
	{"Default (xdg-open)", "xdg-open"},
	{"gedit", "gedit"},
	{"nano", "nano"},
	{"vim", "vim"},
	{"kate", "kate"},
	{"mousepad", "mousepad"},
	{"leafpad", "leafpad"},
	{"pluma", "pluma"},
	{"VSCode", "code"},
	{"Sublime Text", "subl"},
}

// CandidatePrograms returns every hardcoded candidate whose binary is
// actually on PATH, in the original's preference order.
func CandidatePrograms() []Program {
	var out []Program
	for _, c := range candidateCommands {
		if _, err := exec.LookPath(c.command); err == nil {
			out = append(out, Program{Label: c.label, Command: c.command})
		}
	}
	return out
}
