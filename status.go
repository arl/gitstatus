// Package gitstatus provides information about the git status of a working
// tree.
package gitstatus

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
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

	// State indicates the state of the working tree.
	State TreeState

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

var (
	errParseAheadBehind = errors.New("can't parse ahead/behind count")
	errUnexpectedHeader = errors.New("unexpected header format")
	errUnexpectedStatus = errors.New("unexpected git status output")
)

// New returns the Git Status of the current working directory.
func New() (*Status, error) {
	// parse porcelain status
	cmd := exec.Command("git", "status", "-uall", "--porcelain", "--branch", "-z")
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	st := &Status{}
	err := parseCommand(st, cmd)
	if err != nil {
		return nil, wrapError(err, "can't retrieve git status")
	}

	if st.IsInitial {
		// the successive commands require at least one commit.
		return st, nil
	}

	// count stash entries
	cmd = exec.Command("git", "stash", "list")
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	var lc linecount
	err = parseCommand(&lc, cmd)
	if err != nil {
		return nil, wrapError(err, "can't count stash entries")
	}
	st.NumStashed = int(lc)

	// set 'clean working tree' flag
	st.IsClean = st.NumStaged == 0 &&
		st.NumUntracked == 0 &&
		st.NumStashed == 0 &&
		st.NumConflicts == 0

	// sets other special flags and fields.
	cmd = exec.Command("git", "rev-parse", "HEAD", "--git-dir")
	var lines lines
	err = parseCommand(&lines, cmd)
	if err != nil || len(lines) != 2 {
		return nil, wrapError(err, "git rev-parse error")
	}
	st.HEAD = strings.TrimSpace(lines[0])
	st.checkState(strings.TrimSpace(lines[1]))
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

// ReadFrom reads and parses git porcelain status from the given reader, filling
// the corresponding status fields.
func (st *Status) ReadFrom(r io.Reader) (n int64, err error) {
	scan := bufio.NewScanner(r)
	scan.Split(scanNilBytes)
	for scan.Scan() {
		line := scan.Text()
		if len(line) < 2 {
			return 0, errUnexpectedStatus
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
			return 0, err
		}
	}

	if err = scan.Err(); err != nil {
		return
	}

	return
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
		// regular branch[...remote] output, with or without ahead/behind counts
		if len(line) < 4 {
			// branch name is at least one character
			return errUnexpectedHeader
		}
		// check if a remote tracking branch is specified
		pos := strings.Index(line, "...")
		if pos == -1 {
			// we should have the branch name and nothing else, where spaces
			// are not allowed
			if strings.IndexByte(line[3:], ' ') != -1 {
				return errUnexpectedHeader
			}
			st.LocalBranch = line[3:]
		} else {
			st.LocalBranch = line[3:pos]
			st.parseUpstream(line[pos+3:])
		}
	}

	return nil
}

// parseUpstream parses the remote branch name and if present, its divergence
// with local branch (ahead / behind count)
func (st *Status) parseUpstream(s string) error {
	var err error
	pos := strings.IndexByte(s, ' ')
	if pos == -1 {
		st.RemoteBranch = s
		return nil
	}
	st.RemoteBranch = s[:pos]
	s = strings.Trim(s[pos+1:], "[]")

	hasAhead := strings.Contains(s, "ahead")
	hasBehind := strings.Contains(s, "behind")

	switch {
	case hasAhead && hasBehind:
		_, err = fmt.Sscanf(s, "ahead %d, behind %d", &st.AheadCount, &st.BehindCount)
	case hasAhead:
		_, err = fmt.Sscanf(s, "ahead %d", &st.AheadCount)
	case hasBehind:
		_, err = fmt.Sscanf(s, "behind %d", &st.BehindCount)
	default:
		err = fmt.Errorf(`unexpected string "%s"`, s)
	}
	if err != nil {
		return fmt.Errorf("%v: %v", errParseAheadBehind, err)
	}
	return nil
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
		st.State = Bisecting
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
	return wrapErrorf(err, `exec %s "%v"`, cmd.Path, strings.Join(cmd.Args, " "))
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

type lines []string

// ReadFrom reads from r, appending string to lines for each line in r.
func (l *lines) ReadFrom(r io.Reader) (n int64, err error) {
	scan := bufio.NewScanner(r)
	scan.Split(bufio.ScanLines)
	for scan.Scan() {
		*l = append(*l, scan.Text())
	}
	return int64(len(*l)), scan.Err()
}
