package tmux

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/arl/gitstatus"
)

type Config struct {
	Branch     string
	NoRemote   string
	Ahead      string
	Behind     string
	HashPrefix string

	Staged    string
	Conflict  string
	Modified  string
	Untracked string
	Stashed   string
	Clean     string
}

var DefaultCfg = Config{
	NoRemote:   "L",
	Branch:     "⎇  ",
	Staged:     "● ",
	Conflict:   "✖ ",
	Modified:   "✚ ",
	Untracked:  "… ",
	Stashed:    "⚑ ",
	Clean:      "✔",
	Ahead:      "↑·",
	Behind:     "↓·",
	HashPrefix: ":",
}

type Formater struct{ Config }

func (f *Formater) Format(st *gitstatus.Status) (string, error) {
	b := &bytes.Buffer{}

	// overall working tree state
	if st.IsInitial {
		fmt.Fprintf(b, "%s [no commits yet]", st.LocalBranch)
		goto files
	}

	switch st.State {
	case gitstatus.Rebasing:
		fmt.Fprintf(b, "[rebase] %s", f.currentRef(st))
	case gitstatus.AM:
		fmt.Fprintf(b, "[am] %s", f.currentRef(st))
	case gitstatus.AMRebase:
		fmt.Fprintf(b, "[am-rebase] %s", f.currentRef(st))
	case gitstatus.Merging:
		fmt.Fprintf(b, "[merge] %s", f.currentRef(st))
	case gitstatus.CherryPicking:
		fmt.Fprintf(b, "[cherry-pick] %s", f.currentRef(st))
	case gitstatus.Reverting:
		fmt.Fprintf(b, "[revert] %s", f.currentRef(st))
	case gitstatus.Bisecting:
		fmt.Fprintf(b, "[bisect] %s", f.currentRef(st))
	case gitstatus.Default:
		fmt.Fprintf(b, "%s%s", f.Branch, f.currentRef(st))
	}

	if st.RemoteBranch != "" {
		fmt.Fprintf(b, "..%s%s", st.RemoteBranch, f.divergence(st))
	}

files:

	fmt.Fprintf(b, " - ")
	if st.IsClean {
		b.WriteString(f.Clean)
		goto output
	}

	if st.NumStaged != 0 {
		fmt.Fprintf(b, "%s%d", f.Staged, st.NumStaged)
	}
	if st.NumConflicts != 0 {
		fmt.Fprintf(b, "%s%d", f.Conflict, st.NumConflicts)
	}
	if st.NumModified != 0 {
		fmt.Fprintf(b, "%s%d", f.Modified, st.NumModified)
	}
	if st.NumStashed != 0 {
		fmt.Fprintf(b, "%s%d", f.Stashed, st.NumStashed)
	}
	if st.NumUntracked != 0 {
		fmt.Fprintf(b, "%s%d", f.Untracked, st.NumUntracked)
	}

output:

	return b.String(), nil
}

func (f *Formater) currentRef(st *gitstatus.Status) string {
	if st.IsDetached {
		return f.HashPrefix + st.HEAD
	}
	return st.LocalBranch
}

func (f *Formater) divergence(st *gitstatus.Status) string {
	var div string
	if st.BehindCount != 0 {
		div += f.Behind + strconv.Itoa(st.BehindCount)
	}
	if st.AheadCount != 0 {
		div = f.Ahead + strconv.Itoa(st.AheadCount)
	}
	if div != "" {
		return " " + div
	}
	return div
}
