package gitstatus

import (
	"encoding/json"
	"strings"
)

//go:generate stringer -type=TreeState

// TreeState indicates the state of a Git working tree. Its zero-value is
// NormalState.
type TreeState int

const (
	Default       TreeState = iota // Default is state set when the working tree is not in any special state.
	Rebasing                       // Rebasing is the state set when a rebase is in progress, either interactive or manual.
	AM                             // AM is the state set when a git AM is in progress (mailbox patch).
	AMRebase                       // AMRebase is the state set when a git AM rebasing is in progress.
	Merging                        // Merging is the state set when a merge is in progress.
	CherryPicking                  // CherryPicking is the state when a cherry-pick is in progress.
	Reverting                      // Reverting is the state when a revert is in progress.
	Bisecting                      // Bisecting is the state when a bisect is in progress.
)

// MarshalJSON returns the JSON encoding of the tree state.
func (s TreeState) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.ToLower(s.String()))
}
