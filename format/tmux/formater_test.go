package tmux

import (
	"testing"

	"github.com/arl/gitstatus"
	"github.com/stretchr/testify/require"
)

func TestFormater_flags(t *testing.T) {
	tests := []struct {
		name    string
		styles  styles
		symbols symbols
		st      *gitstatus.Status
		want    string
	}{
		{
			name: "clean flag",
			styles: styles{
				Clean: "CleanStyle",
			},
			symbols: symbols{
				Clean: "CleanSymbol",
			},
			st: &gitstatus.Status{
				IsClean: true,
			},
			want: clear + " - CleanStyleCleanSymbol",
		},
		{
			name: "mixed flags",
			styles: styles{
				Modified: "StyleMod",
				Stashed:  "StyleStash",
			},
			symbols: symbols{
				Modified: "SymbolMod",
				Stashed:  "SymbolStash",
			},
			st: &gitstatus.Status{
				NumModified: 2,
				NumStashed:  1,
			},
			want: clear + " - StyleModSymbolMod2 StyleStashSymbolStash1",
		},
		{
			name: "mixed flags 2",
			styles: styles{
				Conflict:  "StyleConflict",
				Untracked: "StyleUntracked",
			},
			symbols: symbols{
				Conflict:  "SymbolConflict",
				Untracked: "SymbolUntracked",
			},
			st: &gitstatus.Status{
				NumConflicts: 42,
				NumUntracked: 17,
			},

			want: clear + " - StyleConflictSymbolConflict42 StyleUntrackedSymbolUntracked17",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &Formater{
				Config: Config{Styles: tc.styles, Symbols: tc.symbols},
				st:     tc.st,
			}
			f.flags()
			require.EqualValues(t, tc.want, f.b.String())
		})
	}
}
