package gitstatus

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatus_parsePorcelain(t *testing.T) {
	tests := []struct {
		name    string
		out     string // git status output
		want    Status
		wantErr error
	}{
		{
			name: "aligned",
			out: "" +
				"# branch.oid 05f8b44d5edc2960eff106e5e780cf83535d0533\n" +
				"# branch.head master\n" +
				"# branch.upstream origin/master\n" +
				"# branch.ab +0 -0\n",
			want: Status{
				LocalBranch:  "master",
				RemoteBranch: "origin/master",
				CommitSHA1:   "05f8b44d5edc2960eff106e5e780cf83535d0533",
			},
		},
		{
			name: "diverged",
			out: "" +
				"# branch.oid 05f8b44d5edc2960eff106e5e780cf83535d0533\n" +
				"# branch.head master\n" +
				"# branch.upstream origin/master\n" +
				"# branch.ab +1 -3\n",
			want: Status{
				LocalBranch:  "master",
				RemoteBranch: "origin/master",
				CommitSHA1:   "05f8b44d5edc2960eff106e5e780cf83535d0533",
				AheadCount:   1,
				BehindCount:  3,
			},
		},
		{
			name: "after git init",
			out: "" +
				"# branch.oid (initial)\n" +
				"# branch.head master\n",
			want: Status{
				LocalBranch: "master",
				IsInitial:   true,
			},
		},
		{
			name: "merge conflicts",
			out: "" +
				"# branch.oid ef7516dfd13efbbd8d64a954dfffc82572c1addf\n" +
				"# branch.head (detached)\n" +
				"D. N... 100644 000000 000000 aaacdd96fd16226816ba2b7a00b2a6a85363dd8b 0000000000000000000000000000000000000000 LICENSE\n" +
				"u UU N... 100644 100644 100644 100644 1113757689eecd5df448b25917fed8ef3ae74705 cc3fdf6f829aeb5c794490158e67ffc33cdeae88 c7da7844b226d19f2f02c1072cf0be97075ca2e8 README.md\n",
			want: Status{
				CommitSHA1: "ef7516dfd13efbbd8d64a954dfffc82572c1addf",
				IsDetached: true,
			},
		},
		{
			name: "untracked with spaces",
			out: "?  1 leading space\n" +
				"? 1 trailing space \n" +
				"? dir/ dir 2 / nested / spaces again \n" +
				"? dir/ nested spaces \n" +
				"? dir/nested\n" +
				"? file1\n",
			want: Status{
				Untracked: []string{
					" 1 leading space",
					"1 trailing space ",
					"dir/ dir 2 / nested / spaces again ",
					"dir/ nested spaces ",
					"dir/nested",
					"file1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Status{}
			r := strings.NewReader(tt.out)
			err := got.parsePorcelain(r)
			assert.Equal(t, err, tt.wantErr)
			assert.Equal(t, *got, tt.want)
		})
	}
}
