//go:build !windows
// +build !windows

package gitstatus

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"golang.org/x/exp/maps"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{"gitstatus": gitstatus}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:      "testdata",
		TestWork: true,
	})
}

// gitstatus creates a Status object based on the current directory and compares
// it with the git status representation in WANT_STATUS environment variable.
func gitstatus() int {
	log.SetPrefix("Error(gitstatus): ")
	log.SetFlags(0)

	status, err := New()
	if err != nil {
		log.Printf("can't create Status object: %v", err)
		return 1
	}

	// Convert args in a 'field name' to 'field value' map.
	kvs := strings.Fields(os.Getenv("WANT_STATUS"))
	fvalues := make(map[string]*regexp.Regexp)
	for _, kv := range kvs {
		fname, fval, ok := strings.Cut(kv, "=")
		if !ok {
			log.Printf("malformed WANT_STATUS key=value pair: %s", kv)
			return 1
		}
		if _, ok := fvalues[fname]; ok {
			log.Printf("duplicate field name in WANT_STATUS: %s", kv)
			return 1
		}
		rx, err := regexp.Compile(fval)
		if err != nil {
			log.Printf("bad regex for field %s: %v", fname, err)
			return 1
		}

		fvalues[fname] = rx
	}

	if err := checkStatusFields(status, fvalues); err != nil {
		log.Printf("failed field check\n%v", err)
		return 1
	}
	return 0
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

func checkStatusFields(status *Status, matches map[string]*regexp.Regexp) error {
	// Keep track of the fields we want to match, so we can check we've actually
	// matched them all.
	set := make(map[string]struct{})
	for fname := range matches {
		set[fname] = struct{}{}
	}

	// Loop on all fields of the given Status instance.
	for _, f := range fieldInfos(*status) {
		delete(set, f.name)
		rx, ok := matches[f.name]
		if !ok {
			// The field was not specified so it has to be the zero value.
			if !f.val.IsZero() {
				return fmt.Errorf("got Status.%s = %v want %v (zero value)", f.name, f.val, reflect.Zero(f.val.Type()))
			}
			continue
		}
		// Match field value with regex provided in test script.
		sval := fmt.Sprintf("%v", f.val)
		if !rx.MatchString(sval) {
			return fmt.Errorf("got Status.%s = %s, doesn't match regular expression %s", f.name, sval, rx)
		}
	}

	if len(set) != 0 {
		return fmt.Errorf("not all Status fields were matched, remaining %+v", maps.Keys(set))
	}

	return nil
}
