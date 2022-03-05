//go:build !windows
// +build !windows

package gitstatus

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"gitstatus": runGitStatus,
	}))
}

// runGitStatus creates a Status object and print its string representation on stdout.
func runGitStatus() int {
	const timeout = 5 * time.Second // Should be more than enough
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	status, err := NewWithContext(ctx)
	if err != nil {
		fmt.Printf("gitstatus error, couldn't create Status object: %v", err)
		return 1
	}
	fmt.Printf("%+v", status)
	return 0
}
