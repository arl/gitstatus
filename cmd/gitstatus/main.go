package main

import (
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

type popdir func() error

func pushdir(dir string) (popdir, error) {
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

	popDir, err := pushdir(dir)
	check(err)
	defer func() {
		check(popDir())
	}()

	st, err := gitstatus.New()
	check(err)

	bb, err := json.MarshalIndent(st, "", " ")
	check(err)
	fmt.Print(string(bb))
}
