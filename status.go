package gitstatus

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"unicode/utf8"
)

// Status represents the status of a Git working tree directory
type Status struct {
	NumAdded     int // NumAdded is the number of files added to the index.
	NumDeleted   int // NumDeleted is the number of files deleted from the index.
	NumUpdated   int // NumUpdated is the number of files updated in index.
	NumRenamed   int // NumRenamed is the number of files renamed in index.
	NumConflicts int // NumConflicts is the number of unmerged files.
	NumUntracked int // NumUntracked is the number of untracked files.

	CommitSHA1   string // CommitSHA1 is the SHA1 of current commit (or empty in initial state)
	LocalBranch  string // LocalBranch is the name of the local branch.
	RemoteBranch string // RemoteBranch is the name of upstream remote branch (tracking).
	AheadCount   int    // AheadCount indicates by how many commits the local branch is ahead of its upstream branch.
	BehindCount  int    // BehindCount indicates by how many commits the local branch is behind its upstream branch.

	IsRebased  bool // IsRebased reports wether a rebase is in progress.
	IsInitial  bool // IsInitial reports wether the working tree is in its initial state (no commit have been performed yet)
	IsDetached bool // IsDetached reports wether HEAD is not associated to any branch (detached).
}

// New returns the Status of the Git working tree 'dir'.
func New(dir string) (*Status, error) {
	cmd := exec.Command("git", "status", "-uall", "--porcelain=2", "--branch", dir)
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "can't run git status")
	}

	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "can't run git status")
	}

	st := &Status{}
	err = st.parsePorcelain(out)
	if err != nil {
		return nil, errors.Wrap(err, "can't parse git status")
	}

	err = cmd.Wait()
	if err != nil {
		return nil, errors.Wrap(err, "can't run git status")
	}

	return st, nil
}

var (
	// branchOID regex matches:
	// # branch.oid <commit> | (initial)
	branchOID = regexp.MustCompile(`^# branch.oid ([a-z0-9]+|\(initial\))$`)

	// branchHEAD regex matches:
	// branch.head <branch> | (detached)
	branchHEAD = regexp.MustCompile(`^# branch.head (.*|\(detached\))$`)

	// branchUpstream regex matches:
	// # branch.upstream <upstream_branch>
	branchUpstream = regexp.MustCompile(`^# branch.upstream (.+)$`)

	// branchAB regex matches:
	// # branch.ab +<ahead> -<behind>
	branchAB = regexp.MustCompile(`^# branch.ab \+([0-9]+)* \-([0-9]+)*$`)
)

func (st *Status) parseHeader(line string) error {
	oid := branchOID.FindStringSubmatch(line)
	if len(oid) == 2 {
		if oid[1] == "(initial)" {
			st.IsInitial = true
		} else {
			st.CommitSHA1 = oid[1]
		}
		return nil
	}

	head := branchHEAD.FindStringSubmatch(line)
	if len(head) == 2 {
		if head[1] == "(detached)" {
			st.IsDetached = true
		} else {
			st.LocalBranch = head[1]
		}
		return nil
	}

	upstream := branchUpstream.FindStringSubmatch(line)
	if len(upstream) == 2 {
		st.RemoteBranch = upstream[1]
		return nil
	}

	ab := branchAB.FindStringSubmatch(line)
	if len(ab) == 3 {
		v, err := strconv.ParseInt(ab[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing branch.ab: %v", err)
		}
		st.AheadCount = int(v)

		v, err = strconv.ParseInt(ab[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing branch.ab: %v", err)
		}
		st.BehindCount = int(v)
		return nil
	}
	return nil
}

// parsePorcelain parses version 2 of Git porcelain status string, ss, and
// fills the corresponding fields of Status.
func (st *Status) parsePorcelain(r io.Reader) error {
	var err error
	scan := bufio.NewScanner(r)
	scan.Split(bufio.ScanLines)
	for scan.Scan() {
		line := scan.Text()
		r, _ := utf8.DecodeRuneInString(line)
		switch r {
		case '#':
			err = st.parseHeader(line)
		case '1':
			// 'ordinary' changed entries
		case '2':
			// renamed or copied entries
		case 'u':
			// unmerged entries
		case '?':
			// untracked items
		case '!':
			// ignored items
		}
		if err != nil {
			return err
		}
	}

	if err := scan.Err(); err != nil {
		return err
	}

	return nil
}
