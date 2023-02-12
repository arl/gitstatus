//go:build !windows
// +build !windows

package gitstatus

import (
	_ "embed"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"golang.org/x/exp/maps"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, nil))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"gitstatus": gitstatus,
		},
	})
}

// runGitStatus creates a Status object and print its string representation on stdout.
// func gitstatus() int
func gitstatus(ts *testscript.TestScript, neg bool, args []string) {
	// Change working directory to that of the script before reading git status.
	// cwd, err := os.Getwd()
	// ts.Check(err)
	ts.Check(os.Chdir(ts.MkAbs("")))

	status, err := New()
	if err != nil {
		ts.Fatalf("gitstatus error, couldn't create Status object: %v", err)
		return
	}

	// Convert args in a 'field name' to 'field value' map.
	fvalues := make(map[string]*regexp.Regexp)
	for _, kv := range args {
		fname, fval, ok := strings.Cut(kv, "=")
		if !ok {
			ts.Fatalf("gitstatus: malformed field name key=value pair: %s", kv)
		}
		if _, ok := fvalues[fname]; ok {
			ts.Fatalf("gitstatus: duplicated field name %s", fname)
		}
		rx, err := regexp.Compile(fval)
		if err != nil {
			ts.Fatalf("gitstatus: bad regex for field %s: %v", fname, err)
		}

		fvalues[fname] = rx
	}

	checkStatusFields(ts, status, fvalues)
}

type fieldInfo struct {
	name string
	val  reflect.Value
}

// fieldInfos fills and returns a slice with the info (field names and values
// after conversion to string) of all fields in s.
func fieldInfos(s any) []fieldInfo {
	var fields []fieldInfo

	var iterFields func(reflect.Value)
	iterFields = func(rv reflect.Value) {
		for i := 0; i < rv.NumField(); i++ {
			ftyp := rv.Type().Field(i)
			if ftyp.Type.Kind() == reflect.Struct {
				iterFields(rv.Field(i))
				continue
			}

			fields = append(fields, fieldInfo{name: ftyp.Name, val: rv.Field(i)})
		}
	}

	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Struct {
		panic("allFields: " + rv.Type().Name() + " is not a struct")
	}

	iterFields(rv)
	return fields
}

func checkStatusFields(ts *testscript.TestScript, s *Status, fieldMatches map[string]*regexp.Regexp) bool {
	// Keep track of matched fields.
	set := make(map[string]struct{})
	for fname := range fieldMatches {
		set[fname] = struct{}{}
	}

	for _, f := range fieldInfos(*s) {
		v, ok := fieldMatches[f.name]
		if !ok {
			// The field was not specified so it has to be the zero value.
			if !f.val.IsZero() {
				ts.Fatalf("got Status.%s = %q want <zero value>", f.name, f.val)
			}
			delete(set, f.name)
			continue
		}

		// Match field value with regex provided in test script.
		sval := fmt.Sprintf("%v", f.val)
		if !v.MatchString(sval) {
			ts.Fatalf("got Status.%s = %s, doesn't match regular expression %s", f.name, sval, v)
		}
		delete(set, f.name)
	}

	if len(set) != 0 {
		ts.Fatalf("not all Status fields were matched, remaining %+v", maps.Keys(set))
	}

	return true
}
