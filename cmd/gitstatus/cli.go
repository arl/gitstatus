package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func check(err error, quiet bool) {
	if err != nil {
		if !quiet {
			fmt.Println("error:", err)
		}
		os.Exit(1)
	}
}

const version = "0.1"
const usage = `gitstatus ` + version + `
Usage: gitstatus [options] [dir]

gitstatus prints the status of a Git working tree.
If directory is not given, it default to the working directory.  

Options:
  -q         be quiet. In case of errors, don't print nothing.
  -fmt       output format, defaults to json.
      json   prints status as a JSON object.
      tmux   prints status as a tmux format string.
`

var errUnknownOutputFormat = errors.New("unknown output format")

func parseOptions() (dir string, format outFormat, quiet bool) {
	fmtOpt := flag.String("fmt", "json", "")
	quietOpt := flag.Bool("q", false, "")
	flag.Usage = func() {
		fmt.Println(usage)
	}
	flag.Parse()
	dir = "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	// format output
	switch *fmtOpt {
	case "json":
		format = outJSON
	case "tmux":
		format = outTmux
	default:
		check(errUnknownOutputFormat, *quietOpt)
	}
	return dir, format, *quietOpt
}
