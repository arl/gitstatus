package main

import (
	"encoding/json"
	"fmt"

	"github.com/arl/gitstatus"
)

type outFormat int

const (
	outJSON outFormat = iota
	outTmux
)

func main() {
	// parse cli options.
	dir, format, quiet := parseOptions()

	// handle directory change.
	if dir != "." {
		popDir, err := pushdir(dir)
		check(err, quiet)
		defer func() {
			check(popDir(), quiet)
		}()
	}

	// retrieve git status.
	st, err := gitstatus.New()
	check(err, quiet)

	// format and print.
	var out string

	switch format {
	case outJSON:
		var buf []byte
		buf, err = json.MarshalIndent(st, "", " ")
		out = string(buf)
	case outTmux:
		out, err = tmuxFormat(st)
	}
	check(err, quiet)
	fmt.Print(out)
}

func tmuxFormat(st *gitstatus.Status) (string, error) {
	panic("not implemented")
}
