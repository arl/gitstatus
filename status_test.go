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
		want    Status
		wantErr error
	}{
		{
			name: "aligned",
			out: porcelainNZT(
				"## master...origin/master",
			),
			want: Status{
				LocalBranch:  "master",
				RemoteBranch: "origin/master",
			},
		},
		{
			name: "no upstream",
			out: porcelainNZT(
				"## master",
			),
			want: Status{
				LocalBranch:  "master",
				RemoteBranch: "",
			},
		},
		{
			name: "ahead",
			out: porcelainNZT(
				"## feature/123/a...upstream/feature/123/a [ahead 3]",
			),
			want: Status{
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
			want: Status{
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
			want: Status{
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
			want: Status{
				LocalBranch: "thisbranch",
				IsInitial:   true,
			},
		},
		{
			name: "detached",
			out: porcelainNZT(
				"## HEAD (no branch)",
			),
			want: Status{
				IsDetached: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Status{}
			r := bytes.NewReader(tt.out)
			_, err := got.ReadFrom(r)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestStatusParseModified(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Status
		wantErr error
	}{
		{
			name: "all cases",
			out: porcelainNZT(
				"## master",
				" M index not updated",
				"MM index updated",
				"AM added to index",
				"RM renamed in index",
				"CM copied in index",
			),
			want: Status{
				LocalBranch: "master",
				NumModified: 5,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Status{}
			r := bytes.NewReader(tt.out)
			_, err := got.ReadFrom(r)
			assert.Equal(t, err, tt.wantErr)
			assert.Equal(t, *got, tt.want)
		})
	}
}

func TestStatusParseConflicts(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Status
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
			want: Status{
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
			want: Status{
				IsDetached:   true,
				NumUntracked: 1,
				NumConflicts: 4,
				NumStaged:    4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Status{}
			r := bytes.NewReader(tt.out)
			_, err := got.ReadFrom(r)
			assert.Equal(t, err, tt.wantErr)
			assert.Equal(t, *got, tt.want)
		})
	}
}

func TestStatusParseUntracked(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Status
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
			want: Status{
				IsDetached:   true,
				NumUntracked: 4,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Status{}
			r := bytes.NewReader(tt.out)
			_, err := got.ReadFrom(r)
			assert.Equal(t, err, tt.wantErr)
			assert.Equal(t, *got, tt.want)
		})
	}
}

func TestStatusParseStaged(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte // git status output
		want    Status
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
				`?? untracked`,
			),
			want: Status{
				IsDetached:   true,
				NumStaged:    6,
				NumUntracked: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Status{}
			r := bytes.NewReader(tt.out)
			_, err := got.ReadFrom(r)
			assert.Equal(t, err, tt.wantErr)
			assert.Equal(t, *got, tt.want)
		})
	}
}

func TestStatusParseMalformed(t *testing.T) {
	tests := []struct {
		name string
		out  []byte // git status output
	}{
		{name: "illegal header", out: porcelainNZT(`##`)},
		{name: "trailing space", out: porcelainNZT(`## branch `)},
		{name: "illformed header", out: porcelainNZT(`## branch [ahead 2`)},
		{name: "illformed header", out: porcelainNZT(`## branch [ahead 2,`)},
		{name: "illformed header", out: porcelainNZT(`## branch [ahead 2, behind 3`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Status{}
			r := bytes.NewReader(tt.out)
			_, err := got.ReadFrom(r)
			assert.Truef(t, err != nil, "want err != nil, got err = %v", err)
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
			r := bytes.NewBufferString(tc.input)
			n, err := lc.ReadFrom(r)
			assert.NoErrorf(t, err, "want err = nil, got err = %s", err)
			assert.Equalf(t, tc.nlines, int64(lc), "nlines != lc, %d != %d, want ==", tc.nlines, lc)
			assert.Equalf(t, tc.nlines, n, "nlines != n, %d != %d, want ==", tc.nlines, n)
		})
	}
}
