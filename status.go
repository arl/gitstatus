// Package gitstatus provides information about the git status of a working
// tree.
package gitstatus

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Status represents the status of a Git working tree directory.
type Status struct {
	NumModified  int // NumModified is the number of modified files.
	NumConflicts int // NumConflicts is the number of unmerged files.
	NumUntracked int // NumUntracked is the number of untracked files.
	NumStaged    int // NumStaged is the number of staged files.
	NumStashed   int // NumStashed is the number of stash entries.

	HEAD         string // HEAD is the SHA1 of current commit (empty in initial state).
	LocalBranch  string // LocalBranch is the name of the local branch.
	RemoteBranch string // RemoteBranch is the name of upstream remote branch (tracking).
	AheadCount   int    // AheadCount reports by how many commits the local branch is ahead of its upstream branch.
	BehindCount  int    // BehindCount reports by how many commits the local branch is behind its upstream branch.

	// State reports the current working tree state.
	State WorkingTreeState

	// IsInitial reports wether the working tree is in its initial state (no
	// commit have been performed yet).
	IsInitial bool

	// IsDetached reports wether HEAD is not associated to any branch
	// (detached).
	IsDetached bool

	// IsClean reports wether the working tree is in a clean state (i.e empty
	// staging area, no conflicts, no stash entries, no untracked files).
	IsClean bool
}

// SpecialState indicates the state of a Git working tree. Its zero-value is
// NormalState.
type WorkingTreeState int

const (
	Default       WorkingTreeState = iota // Default is state set when the working tree is not in any special state.
	Rebasing                              // Rebasing is the state set when a rebase is in progress, either interactive or manual.
	AM                                    // AM is the state set when a git AM is in progress (mailbox patch).
	AMRebase                              // AMRebase is the state set when a git AM rebasing is in progress.
	Merging                               // Merging is the state set when a merge is in progress.
	CherryPicking                         // CherryPicking is the state when a cherry-pick is in progress.
	Reverting                             // Reverting is the state when a revert is in progress.
	Bissecting                            // Bissecting is the state when a bissect is in progress.
)

// New returns the Git Status of the current working directory.
func New() (*Status, error) {
	// parse porcelain status
	cmd := exec.Command("git", "status", "-uall", "--porcelain", "--branch", "-z")
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	st := &Status{}
	err := parseCommand(st, cmd)
	if err != nil {
		return nil, errors.Wrap(err, "can't retrieve git status")
	}

	// count stash entries
	cmd = exec.Command("git", "stash", "list")
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	var lc linecount
	err = parseCommand(&lc, cmd)
	if err != nil {
		return nil, errors.Wrap(err, "can't count stash entries")
	}
	st.NumStashed = int(lc)

	// set 'clean working tree' flag
	st.IsClean = st.NumStaged == 0 &&
		st.NumUntracked == 0 &&
		st.NumStashed == 0 &&
		st.NumConflicts == 0

	// sets other special flags and fields.
	cmd = exec.Command("git", "rev-parse", "--git-dir")
	gitdir, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "can't retrieve git-dir")
	}
	st.checkState(string(gitdir))
	return st, nil
}

// scanNilBytes is a bufio.SplitFunc function used to tokenize the input with
// nil bytes. The last byte should always be a nil byte or scanNilBytes returns
// an error.
func scanNilBytes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		// We have a full nil-terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we would have a final not ending with a nil byte, we
	// won't allow that.
	if atEOF {
		return 0, nil, errors.New("last line doesn't end with a nil byte")
	}
	// Request more data.
	return 0, nil, nil
}

// TODO: find an easier regex?
var upstreamRx = regexp.MustCompile(`^([[:print:]]+?)(?: \[ahead ([[:digit:]]+), behind ([[:digit:]]+)\]){0,1}$`)

// parses upstream branch name and if present, branch divergence.
func (st *Status) parseUpstream(s string) error {
	res := upstreamRx.FindStringSubmatch(s)
	if len(res) != 4 {
		return fmt.Errorf(`malformed upstream branch: "%s"`, s)
	}

	st.RemoteBranch = res[1]

	var err error
	if res[2] != "" && res[3] != "" {
		st.AheadCount, err = strconv.Atoi(res[2])
		err = errors.Wrap(err, "ahead count")
		st.BehindCount, err = strconv.Atoi(res[3])
		err = errors.Wrap(err, "behind count")
	}
	return err
}

func (st *Status) parseHeader(line string) error {
	const (
		initialPrefix = "## No commits yet on "
		detachedStr   = "## HEAD (no branch)"
	)
	switch {
	case line == detachedStr:
		st.IsDetached = true
	case strings.HasPrefix(line, initialPrefix):
		st.IsInitial = true
		st.LocalBranch = line[len(initialPrefix):]
	default:
		pos := strings.Index(line, "...")
		if pos == -1 {
			// no remote tracking
			st.LocalBranch = line[3:]
		} else {
			st.LocalBranch = line[3:pos]
			st.parseUpstream(line[pos+3:])
		}
	}

	return nil
}

// ReadFrom reads and parses git porcelain status from the given reader, filling
// the corresponding status fields.
func (st *Status) ReadFrom(r io.Reader) (n int64, err error) {
	scan := bufio.NewScanner(r)
	scan.Split(scanNilBytes)
	for scan.Scan() {
		line := scan.Text()
		if len(line) < 2 {
			panic("unknown status line")
		}

		first, second := line[0], line[1]
		switch {
		case first == '#' && second == '#':
			err = st.parseHeader(line)
		case second == 'M':
			st.NumModified++
		case first == 'U':
			st.NumConflicts++
		case first == '?' && second == '?':
			st.NumUntracked++
		default:
			st.NumStaged++
		}

		if err != nil {
			return
		}
	}

	if err = scan.Err(); err != nil {
		return
	}

	return
}

// returns true if the path made of the given components exists and is readable.
func exists(components ...string) bool {
	_, err := os.Stat(path.Join(components...))
	return err == nil
}

// checkState checks the current state of the working tree and sets at most one
// special state flag accordingly.
func (st *Status) checkState(gitdir string) {
	st.State = Default
	// from: git/contrib/completion/git-prompt.sh
	switch {
	case exists(gitdir, "rebase-merge"):
		st.State = Rebasing
	case exists(gitdir, "rebase-apply"):
		switch {
		case exists(gitdir, "rebase-apply", "rebasing"):
			st.State = Rebasing
		case exists(gitdir, "rebase-apply", "applying"):
			st.State = AM
		default:
			st.State = AMRebase
		}
	case exists(gitdir, "MERGE_HEAD"):
		st.State = Merging
	case exists(gitdir, "CHERRY_PICK_HEAD"):
		st.State = CherryPicking
	case exists(gitdir, "REVERT_HEAD"):
		st.State = Reverting
	case exists(gitdir, "BISECT_LOG"):
		st.State = Bissecting
	}
}

// parseCommand runs cmd and parses its output through dst.
//
// A pipe is attached to the process standard output, that is redirected to dst.
// The command is ran from the current working directory.
func parseCommand(dst io.ReaderFrom, cmd *exec.Cmd) error {
	out, err := cmd.StdoutPipe()
	if err != nil {
		return errCmd(err, cmd)
	}

	err = cmd.Start()
	if err != nil {
		return errCmd(err, cmd)
	}

	_, err = dst.ReadFrom(out)
	if err != nil {
		return errCmd(err, cmd)
	}

	return errCmd(cmd.Wait(), cmd)
}

func errCmd(err error, cmd *exec.Cmd) error {
	if err != nil {
		return errors.Wrapf(err, "exec %s %v", cmd.Path, cmd.Args)
	}
	return nil
}

type linecount int

// ReadFrom reads from r, counting the number of lines.
func (lc *linecount) ReadFrom(r io.Reader) (n int64, err error) {
	scan := bufio.NewScanner(r)
	scan.Split(bufio.ScanLines)
	for scan.Scan() {
		*lc++
	}
	return int64(*lc), scan.Err()
}
