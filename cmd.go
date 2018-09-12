package gitstatus

import (
	"io"
	"os"
	"os/exec"
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

	err := parseCommand(r, cmd)
	if err != nil {
		return err
	}
	return nil
}
