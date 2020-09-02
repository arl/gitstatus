package gitstatus

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

var env []string

type parserFrom interface {
	parseFrom(r io.Reader) error
}

func runAndParse(ctx context.Context, p parserFrom, prog string, args ...string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if env == nil {
		// cache env
		env = []string{
			"LC_ALL=C",             // override any user-specific localization
			"GIT_OPTIONAL_LOCKS=0", // disable operations requiring locks
		}

		home, ok := os.LookupEnv("HOME")
		if ok {
			env = append(env, "HOME="+home)
		}
	}
	// parse porcelain status
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Env = env

	buf, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("exec %s '%v': %w", cmd.Path, strings.Join(args, " "), err)
	}

	rbuf := bytes.NewReader(buf)
	if err := p.parseFrom(rbuf); err != nil {
		return fmt.Errorf("exec %s '%v': %w", cmd.Path, strings.Join(args, " "), err)
	}

	return nil
}
