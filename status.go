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

// Status represents the status of a Git working tree directory
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

	IsRebasing      bool // IsRebasing reports wether a rebase is in progress, either interactive or manual.
	IsAM            bool // IsAM reports wether a git AM is in progress (mailbox patch).
	IsAMRebase      bool // IsAMRebase reports wether a git AM rebasing is in progress.
	IsMerging       bool // IsMerging reports wether a merge is in progress.
	IsCherryPicking bool // IsCherryPicking reports wether a cherry-pick is in progress.
	IsReverting     bool // IsReverting reports wether a revert is in progress.
	IsBissecting    bool // IsBissecting reports wether a bissect is in progress.

	IsInitial  bool // IsInitial reports wether the working tree is in its initial state (no commit have been performed yet)
	IsDetached bool // IsDetached reports wether HEAD is not associated to any branch (detached).

	IsClean bool // IsClean reports wether the working tree is in a clean state (i.e empty staging area, no conflicts, no stash entries, no untracked files)
}

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
		return 0, nil, errors.New("last line doesn't end with a null byte")
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
	var err error
	st.RemoteBranch = res[1]
	if res[2] != "" {
		st.AheadCount, err = strconv.Atoi(res[2])
		if err != nil {
			return errors.Wrap(err, "ahead count")
		}
	}
	if res[3] != "" {
		st.BehindCount, err = strconv.Atoi(res[3])
		if err != nil {
			return errors.Wrap(err, "behind count")
		}
	}
	return nil
}

func (st *Status) parseHeader(line string) error {
	const (
		initialPrefix = "## No commits yet on "
		detachedStr   = "## HEAD (no branch)"
	)
	if line == detachedStr {
		st.IsDetached = true
	} else if strings.HasPrefix(line, initialPrefix) {
		st.IsInitial = true
		st.LocalBranch = line[len(initialPrefix):]
	} else {
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
	// from: git/contrib/completion/git-prompt.sh
	switch {
	case exists(gitdir, "rebase-merge"):
		st.IsRebasing = true
	case exists(gitdir, "rebase-apply"):
		switch {
		case exists(gitdir, "rebase-apply", "rebasing"):
			st.IsRebasing = true
		case exists(gitdir, "rebase-apply", "applying"):
			st.IsAM = true
		default:
			st.IsAMRebase = true
		}
	case exists(gitdir, "MERGE_HEAD"):
		st.IsMerging = true
	case exists(gitdir, "CHERRY_PICK_HEAD"):
		st.IsCherryPicking = true
	case exists(gitdir, "REVERT_HEAD"):
		st.IsReverting = true
	case exists(gitdir, "BISECT_LOG"):
		st.IsBissecting = true
	}
}

// parseCommand runs cmd and parses its output through dst.
//
// A pipe is attached to the process standard output, that is redirected to dst.
// The command is ran from the current working directory.
func parseCommand(dst io.ReaderFrom, cmd *exec.Cmd) error {
	out, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrapf(err, "exec %s %v", cmd.Path, cmd.Args)
	}

	err = cmd.Start()
	if err != nil {
		return errors.Wrapf(err, "exec %s %v", cmd.Path, cmd.Args)
	}

	_, err = dst.ReadFrom(out)
	if err != nil {
		return errors.Wrapf(err, "exec %s %v", cmd.Path, cmd.Args)
	}

	err = cmd.Wait()
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
