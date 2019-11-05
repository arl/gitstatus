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

// parseCommand runs cmd and parses its output through dst.
//
// A pipe is attached to the process standard output, that is redirected to dst.
// The command is ran from the current working directory.
func parseCommand(dst io.ReaderFrom, cmd *exec.Cmd) error {
	out, err := cmd.StdoutPipe()
	if err != nil {
		return errCmd(err, cmd)
	}

	err = cmd.Start()
	if err != nil {
		return errCmd(err, cmd)
	}

	_, err = dst.ReadFrom(out)
	if err != nil {
		return errCmd(err, cmd)
	}

	return errCmd(cmd.Wait(), cmd)
}
