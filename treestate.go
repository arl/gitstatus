package gitstatus

import (
	"encoding/json"
	"os"
	"path"
	"strings"
)

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=TreeState

// TreeState indicates the state of a Git working tree. Its zero-value is
// NormalState.
type TreeState int

const (
	// Default is state set when the working tree is not in any special state.
	Default TreeState = iota

	// Rebasing is the state set when a rebase is in progress, either
	// interactive or manual.
	Rebasing

	// AM is the state set when a git AM is in progress (mailbox patch).
	AM

	// AMRebase is the state set when a git AM rebasing is in progress.
	AMRebase

	// Merging is the state set when a merge is in progress.
	Merging

	// CherryPicking is the state when a cherry-pick is in progress.
	CherryPicking

	// Reverting is the state when a revert is in progress.
	Reverting

	// Bisecting is the state when a bisect is in progress.
	Bisecting
)

// MarshalJSON returns the JSON encoding of the tree state.
func (s TreeState) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.ToLower(s.String()))
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

// setState checks the current state of the working tree and sets at most one
// special state flag accordingly.
func treeStateFromDir(gitdir string) TreeState {
	ts := Default
	// from: git/contrib/completion/git-prompt.sh
	switch {
	case exists(gitdir, gitDirRebaseMerge):
		ts = Rebasing
	case exists(gitdir, gitDirRebaseApply):
		switch {
		case exists(gitdir, gitDirRebaseApply, gitDirRebasing):
			ts = Rebasing
		case exists(gitdir, gitDirRebaseApply, gitDirApplying):
			ts = AM
		default:
			ts = AMRebase
		}
	case exists(gitdir, gitDirMergeHead):
		ts = Merging
	case exists(gitdir, gitDirCherryPickHead):
		ts = CherryPicking
	case exists(gitdir, gitDirRevertHead):
		ts = Reverting
	case exists(gitdir, gitDirBisectLog):
		ts = Bisecting
	}

	return ts
}

// Returns true if the path made of the given components exists and is readable.
func exists(components ...string) bool {
	_, err := os.Stat(path.Join(components...))
	return err == nil
}
