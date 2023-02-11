package gitstatus

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func porcelainNZT(lines ...string) []byte {
	return append([]byte(strings.Join(lines, "\x00")), 0)
}

func TestStatusParseHeaders(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Porcelain
		wantErr error
	}{
		{
			name: "aligned",
			out: porcelainNZT(
				"## master...origin/master",
			),
			want: Porcelain{
				LocalBranch:  "master",
				RemoteBranch: "origin/master",
			},
		},
		{
			name: "no upstream",
			out: porcelainNZT(
				"## master",
			),
			want: Porcelain{
				LocalBranch:  "master",
				RemoteBranch: "",
			},
		},
		{
			name: "ahead",
			out: porcelainNZT(
				"## feature/123/a...upstream/feature/123/a [ahead 3]",
			),
			want: Porcelain{
				LocalBranch:  "feature/123/a",
				RemoteBranch: "upstream/feature/123/a",
				AheadCount:   3,
			},
		},
		{
			name: "behind",
			out: porcelainNZT(
				"## feature/123/a...upstream/feature/123/a [behind 2]",
			),
			want: Porcelain{
				LocalBranch:  "feature/123/a",
				RemoteBranch: "upstream/feature/123/a",
				BehindCount:  2,
			},
		},
		{
			name: "diverged",
			out: porcelainNZT(
				"## feature/123/a...upstream/feature/123/a [ahead 26, behind 2]",
			),
			want: Porcelain{
				LocalBranch:  "feature/123/a",
				RemoteBranch: "upstream/feature/123/a",
				AheadCount:   26,
				BehindCount:  2,
			},
		},
		{
			name: "initial",
			out: porcelainNZT(
				"## No commits yet on thisbranch",
			),
			want: Porcelain{
				LocalBranch: "thisbranch",
				IsInitial:   true,
			},
		},
		{
			name: "detached",
			out: porcelainNZT(
				"## HEAD (no branch)",
			),
			want: Porcelain{
				IsDetached: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Porcelain{}
			assert.Equal(t, tt.wantErr, got.parseFrom(bytes.NewReader(tt.out)))
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestStatusParseModified(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Porcelain
		wantErr error
	}{
		{
			name: "all cases",
			out: porcelainNZT(
				"## master",
				" M index not updated",
				"MM staged and modified",
				"AM added to index",
				"RM renamed in index",
				"CM copied in index",
				" D deleted in index",
			),
			want: Porcelain{
				LocalBranch: "master",
				NumModified: 6,
				NumStaged:   3,
			},
		},
		{
			name: "issue-11",
			out: porcelainNZT(
				"## issue-11/staged-hunks",
				"MM gitmux.go",
			),
			want: Porcelain{
				LocalBranch: "issue-11/staged-hunks",
				NumModified: 1,
				NumStaged:   1,
			},
		},
		{
			name: "added then modified",
			out: porcelainNZT(
				"## main",
				"AM file",
			),
			want: Porcelain{
				LocalBranch: "main",
				NumModified: 1,
				NumStaged:   1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Porcelain{}
			assert.Equal(t, tt.wantErr, got.parseFrom(bytes.NewReader(tt.out)))
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestStatusParseConflicts(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Porcelain
		wantErr error
	}{
		{
			name: "conflict 1",
			out: porcelainNZT(
				"## HEAD (no branch)",
				"UD unmerged, deleted by them",
				"UA unmerged, added by them",
				"UU unmerged, both modified",
			),
			want: Porcelain{
				IsDetached:   true,
				NumConflicts: 3,
			},
		},
		{
			name: "conflict 2",
			out: porcelainNZT(
				`## HEAD (no branch)`,
				`UU example/sudoku/main.go`,
				`M  example/sudoku/operators_test.go`,
				`DU pkg/engine/engine.go`,
				`UU pkg/mt19937/mt19937.go`,
				`UU pkg/mt19937/mt19937_test.go`,
				`R  random/utils_test.go -> pkg/mt19937/utils_test.go`,
				`D  random/mersenne_twister.go`,
				`D  random/mersenne_twister_test.go`,
				`?? TODO`,
			),
			want: Porcelain{
				IsDetached:   true,
				NumUntracked: 1,
				NumConflicts: 4,
				NumStaged:    4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Porcelain{}
			assert.Equal(t, tt.wantErr, got.parseFrom(bytes.NewReader(tt.out)))
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestStatusParseUntracked(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Porcelain
		wantErr error
	}{
		{
			name: "all cases",
			out: porcelainNZT(
				`## HEAD (no branch)`,
				`?? blabla`,
				`?? "dir1/dir2/nested with\ttab"`,
				`?? "dir1/dir2/nested with backslash\\"`,
				`?? "dir1/dir2/nested with carrier \nreturn"`,
			),
			want: Porcelain{
				IsDetached:   true,
				NumUntracked: 4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Porcelain{}
			assert.Equal(t, tt.wantErr, got.parseFrom(bytes.NewReader(tt.out)))
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestStatusParseStaged(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Porcelain
		wantErr error
	}{
		{
			name: "all cases",
			out: porcelainNZT(
				`## HEAD (no branch)`,
				`A  dir1/dir2/nested`,
				`A  "dir1/dir2/nested with\ttab"`,
				`A  "dir1/dir2/nested with backslash\\"`,
				`A  "dir1/dir2/nested with carrier \nreturn"`,
				`M  fileb`,
				`A  newfile`,
				`D  deleted`,
				`?? untracked`,
			),
			want: Porcelain{
				IsDetached:   true,
				NumStaged:    7,
				NumUntracked: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Porcelain{}
			assert.Equal(t, tt.wantErr, got.parseFrom(bytes.NewReader(tt.out)))
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestStatusParseMalformed(t *testing.T) {
	tests := []struct {
		name string
		out  []byte // git status output
	}{
		{name: "trailing space", out: porcelainNZT(`## branch `)},
		{name: "illformed header", out: porcelainNZT(`## branch [ahead 2`)},
		{name: "illformed header", out: porcelainNZT(`## branch [ahead 2,`)},
		{name: "illformed header", out: porcelainNZT(`## branch [ahead 2, behind 3`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Porcelain{}
			assert.Error(t, got.parseFrom(bytes.NewReader(tt.out)))
		})
	}
}

func TestLineCount(t *testing.T) {
	tests := []struct {
		input  string
		nlines int64 // expected number of lines
	}{
		{input: "", nlines: 0},
		{input: "\n", nlines: 1},
		{input: "\n\n", nlines: 2},
		{input: "\r\n", nlines: 1},
		{input: "\r\n\r\n", nlines: 2},
	}
	for _, tc := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			var lc linecount
			assert.NoError(t, lc.parseFrom(bytes.NewBufferString(tc.input)))
			assert.EqualValues(t, tc.nlines, lc)
		})
	}
}

func Test_extractShortStat(t *testing.T) {
	tests := []struct {
		line       string
		insertions int
		deletions  int
	}{
		{line: `2 files changed, 55 insertions(+), 1 deletion(-)`, insertions: 55, deletions: 1},
		{line: `2 files changed, 55 insertions(+), 3 deletions(-)`, insertions: 55, deletions: 3},
		{line: `2 files changed, 1 insertion(+), 1 deletion(-)`, insertions: 1, deletions: 1},
		{line: `2 files changed, 1 insertion(+), 23 deletions(-)`, insertions: 1, deletions: 23},
		{line: `1 file changed, 14 insertions(+)`, insertions: 14},
		{line: `1 file changed, 1 insertion(+)`, insertions: 1},
		{line: `1 file changed, 53 deletions(+)`, deletions: 53},
		{line: `1 file changed, 1 deletion(+)`, deletions: 1},
		{line: ``, insertions: 0, deletions: 0},
	}

	for _, tt := range tests {
		insertions, deletions, err := extractShortStat([]byte(tt.line))
		if err != nil {
			t.Fatal(err)
		}
		if tt.insertions != insertions || tt.deletions != deletions {
			t.Errorf("git diff --shortstat\n%s\n got +%d/-%d want +%d/-%d", tt.line, insertions, deletions, tt.insertions, tt.deletions)
		}
	}
}
