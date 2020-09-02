// Package gitstatus provides information about the git status of a working
// tree.
package gitstatus

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
)

// Status represents the status of a Git working tree directory.
type Status struct {
	Porcelain

	NumStashed int // NumStashed is the number of stash entries.

	HEAD string // HEAD is the shortened SHA1 of current commit (empty in initial state).

	// State indicates the state of the working tree.
	State TreeState

	// IsClean reports wether the working tree is in a clean state (i.e empty
	// staging area, no conflicts, no stash entries, no untracked files).
	IsClean bool
}

// Porcelain holds the Git status variables extracted from calling git status --porcelain.
type Porcelain struct {
	NumModified  int // NumModified is the number of modified files.
	NumConflicts int // NumConflicts is the number of unmerged files.
	NumUntracked int // NumUntracked is the number of untracked files.
	NumStaged    int // NumStaged is the number of staged files.

	// IsDetached reports wether HEAD is not associated to any branch
	// (detached).
	IsDetached bool

	// IsInitial reports wether the working tree is in its initial state (no
	// commit have been performed yet).
	IsInitial bool

	LocalBranch  string // LocalBranch is the name of the local branch.
	RemoteBranch string // RemoteBranch is the name of upstream remote branch (tracking).
	AheadCount   int    // AheadCount reports by how many commits the local branch is ahead of its upstream branch.
	BehindCount  int    // BehindCount reports by how many commits the local branch is behind its upstream branch.
}

var (
	errParseAheadBehind = errors.New("can't parse ahead/behind count")
	errUnexpectedHeader = errors.New("unexpected header format")
)

// New returns the Git Status of the current working directory.
func New() (*Status, error) { return new(context.Background()) }

// NewWithContext is likes New but includes a context.
//
// The provided context is used to stop retrieving git status if the context
// becomes done before all calls to git have completed.
func NewWithContext(ctx context.Context) (*Status, error) { return new(ctx) }

func new(ctx context.Context) (*Status, error) {
	st := &Status{}

	err := runAndParse(ctx, &st.Porcelain, "git", "status", "--porcelain", "--branch", "-z")
	if err != nil {
		return nil, err
	}

	if st.IsInitial {
		// the successive commands require at least one commit.
		return st, nil
	}

	// count stash entries
	var lc linecount

	err = runAndParse(ctx, &lc, "git", "stash", "list")
	if err != nil {
		return nil, err
	}

	// sets other special flags and fields.
	var lines lines

	err = runAndParse(ctx, &lines, "git", "rev-parse", "--git-dir", "--short", "HEAD")
	if err != nil || len(lines) != 2 {
		return nil, err
	}

	st.checkState(strings.TrimSpace(lines[0]))
	st.HEAD = strings.TrimSpace(lines[1])
	st.NumStashed = int(lc)
	st.IsClean = st.NumStaged+st.NumConflicts+st.NumModified+st.NumStashed+st.NumUntracked == 0

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

var fileStatusRx = regexp.MustCompile(`^(##|[ MADRCU?!]{2}) .*$`)

// parseStatus parses porcelain status and fills it with r.
func (p *Porcelain) parseFrom(r io.Reader) error {
	scan := bufio.NewScanner(r)
	scan.Split(scanNilBytes)

	var err error
	for scan.Scan() {
		line := scan.Text()
		if !fileStatusRx.MatchString(line) {
			continue
		}

		first, second := line[0], line[1]

		switch {
		case first == '#' && second == '#':
			err = p.parseHeader(line)
		case first == 'U', second == 'U':
			p.NumConflicts++
		case second == 'M', second == 'D':
			p.NumModified++
		case first == '?' && second == '?':
			p.NumUntracked++
		default:
			p.NumStaged++
		}

		if err != nil {
			return err
		}
	}

	return scan.Err()
}

func (p *Porcelain) parseHeader(line string) error {
	const (
		initialPrefix = "## No commits yet on "
		detachedStr   = "## HEAD (no branch)"
	)

	switch {
	case line == detachedStr:
		p.IsDetached = true
	case strings.HasPrefix(line, initialPrefix):
		p.IsInitial = true
		p.LocalBranch = line[len(initialPrefix):]
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
			p.LocalBranch = line[3:]
		} else {
			p.LocalBranch = line[3:pos]
			p.parseUpstream(line[pos+3:])
		}
	}

	return nil
}

// parseUpstream parses the remote branch name and if present, its divergence
// with local branch (ahead / behind count)
func (p *Porcelain) parseUpstream(s string) error {
	var err error

	pos := strings.IndexByte(s, ' ')
	if pos == -1 {
		p.RemoteBranch = s
		return nil
	}
	p.RemoteBranch = s[:pos]
	s = strings.Trim(s[pos+1:], "[]")

	hasAhead := strings.Contains(s, "ahead")
	hasBehind := strings.Contains(s, "behind")

	switch {
	case hasAhead && hasBehind:
		_, err = fmt.Sscanf(s, "ahead %d, behind %d", &p.AheadCount, &p.BehindCount)
	case hasAhead:
		_, err = fmt.Sscanf(s, "ahead %d", &p.AheadCount)
	case hasBehind:
		_, err = fmt.Sscanf(s, "behind %d", &p.BehindCount)
	default:
		err = fmt.Errorf(`unexpected string "%s"`, s)
	}

	if err != nil {
		return fmt.Errorf("%v: %w", errParseAheadBehind, err)
	}
	return nil
}

// returns true if the path made of the given components exists and is readable.
func exists(components ...string) bool {
	_, err := os.Stat(path.Join(components...))
	return err == nil
}

const (
	gitDirRebaseMerge    = "rebase-merge"
	gitDirRebaseApply    = "rebase-apply"
	gitDirRebasing       = "rebasing"
	gitDirApplying       = "applying"
	gitDirMergeHead      = "MERGE_HEAD"
	gitDirCherryPickHead = "CHERRY_PICK_HEAD"
	gitDirRevertHead     = "REVERT_HEAD"
	gitDirBisectLog      = "BISECT_LOG"
)

// checkState checks the current state of the working tree and sets at most one
// special state flag accordingly.
func (st *Status) checkState(gitdir string) {
	st.State = Default
	// from: git/contrib/completion/git-prompt.sh
	switch {
	case exists(gitdir, gitDirRebaseMerge):
		st.State = Rebasing
	case exists(gitdir, gitDirRebaseApply):
		switch {
		case exists(gitdir, gitDirRebaseApply, gitDirRebasing):
			st.State = Rebasing
		case exists(gitdir, gitDirRebaseApply, gitDirApplying):
			st.State = AM
		default:
			st.State = AMRebase
		}
	case exists(gitdir, gitDirMergeHead):
		st.State = Merging
	case exists(gitdir, gitDirCherryPickHead):
		st.State = CherryPicking
	case exists(gitdir, gitDirRevertHead):
		st.State = Reverting
	case exists(gitdir, gitDirBisectLog):
		st.State = Bisecting
	}
}

type linecount int

// parseFrom counts the number of lines by reading from r.
func (lc *linecount) parseFrom(r io.Reader) error {
	scan := bufio.NewScanner(r)
	scan.Split(bufio.ScanLines)

	for scan.Scan() {
		*lc++
	}

	return scan.Err()
}

type lines []string

// parseFrom appends to itself the lines it finds by reading r.
func (l *lines) parseFrom(r io.Reader) error {
	scan := bufio.NewScanner(r)
	scan.Split(bufio.ScanLines)

	for scan.Scan() {
		*l = append(*l, scan.Text())
	}

	return scan.Err()
}
