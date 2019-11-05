package gitstatus

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

var env []string

func runAndParse(r io.ReaderFrom, prog string, args ...string) error {
	if env == nil {
		// cache env
		env = []string{"LC_ALL=C"}
		home, ok := os.LookupEnv("HOME")
		if ok {
			env = append(env, "HOME="+home)
		}
	}
	// parse porcelain status
	cmd := exec.Command(prog, args...)
	cmd.Env = env
	buf, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("exec %s '%v': %w", cmd.Path, strings.Join(args, " "), err)
	}
	rbuf := bytes.NewReader(buf)
	if _, err := r.ReadFrom(rbuf); err != nil {
		return fmt.Errorf("exec %s '%v': %w", cmd.Path, strings.Join(args, " "), err)
	}
	return nil
}
