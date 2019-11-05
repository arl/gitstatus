package gitstatus

import (
	"bytes"
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
	buf, err := cmd.Output()
	rbuf := bytes.NewReader(buf)

	_, err = r.ReadFrom(rbuf)
	return errCmd(err, cmd)
}
