package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/arl/gitstatus"
)

func check(err error, args ...interface{}) {
	if err != nil {
		fmt.Println("error:", err, args)
		os.Exit(1)
	}
}

type popDirFunc func() error

func pushDir(dir string) (popDirFunc, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	err = os.Chdir(dir)
	if err != nil {
		return nil, err
	}

	return func() error {
		return os.Chdir(pwd)
	}, nil
}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	popDir, err := pushDir(dir)
	check(err)
	defer func() { check(popDir()) }()

	st, err := gitstatus.New(dir)
	check(err)

	jb, err := json.Marshal(st)
	bb := bytes.Buffer{}
	json.Indent(&bb, jb, "", "  ")
	check(err)
	fmt.Print(bb.String())
}
