package gitstatus

import (
	"encoding/json"
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
