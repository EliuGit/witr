package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pranshuparmar/witr/internal/output"
	"github.com/pranshuparmar/witr/pkg/model"
)

var updateContract = flag.Bool("update", false, "rewrite testdata/json_contract.golden")

// --json is a contract (README, "7.4 JSON Output"): field names and types stay
// stable, and releases only add fields. This pins every field of every JSON
// shape witr prints, built from fully populated values, so a rename, removal
// or type change fails here. After deliberately adding a field, run
//
//	go test ./internal/app -run TestJSONContract -update
//
// and commit the updated golden file.
func TestJSONContract(t *testing.T) {
	var r model.Result
	contractFill(reflect.ValueOf(&r).Elem())
	var cm model.ContainerMatch
	contractFill(reflect.ValueOf(&cm).Elem())
	target := model.Target{Type: model.TargetName, Value: "x"}

	shapes := []struct {
		name string
		json func() (string, error)
	}{
		{"--json (full report)", func() (string, error) { return output.ToJSON(r) }},
		{"--short --json", func() (string, error) { return output.ToShortJSON(r) }},
		{"--tree --json", func() (string, error) { return output.ToTreeJSON(r) }},
		{"--warnings --json", func() (string, error) { return output.ToWarningsJSON(r) }},
		{"--env --json", func() (string, error) { return output.ToEnvJSON(r) }},
		{"container view", func() (string, error) { return output.ContainerFallbackToJSON("container x", &cm, "note") }},
		{"failed lookup", func() (string, error) { return jsonErrorEntry(target, "error"), nil }},
		{"ambiguous process", func() (string, error) {
			return jsonMatchEntry(target, "error", []processMatch{{PID: 1, Command: "x", Cmdline: "x"}}), nil
		}},
		{"ambiguous container", func() (string, error) {
			return jsonMatchEntry(target, "error", []*model.ContainerMatch{&cm}), nil
		}},
	}

	var b strings.Builder
	for _, s := range shapes {
		out, err := s.json()
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		var v any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("%s: invalid JSON: %v", s.name, err)
		}
		paths := map[string]string{}
		contractPaths(v, "", paths)
		keys := make([]string, 0, len(paths))
		for k := range paths {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(&b, "# %s\n", s.name)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s %s\n", k, paths[k])
		}
		b.WriteString("\n")
	}
	got := b.String()

	golden := filepath.Join("testdata", "json_contract.golden")
	if *updateContract {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	// A Windows checkout may convert the golden file to CRLF line endings.
	want := strings.ReplaceAll(string(data), "\r\n", "\n")
	if got == want {
		return
	}
	gotFields, wantFields := contractFields(got), contractFields(want)
	for _, f := range wantFields {
		if !slices.Contains(gotFields, f) {
			t.Errorf("removed or changed, a breaking change: %s", f)
		}
	}
	for _, f := range gotFields {
		if !slices.Contains(wantFields, f) {
			t.Errorf("added (run with -update if deliberate): %s", f)
		}
	}
}

// contractFields lists the golden file's fields as "<shape>: <path> <type>".
func contractFields(golden string) []string {
	var fields []string
	shape := ""
	for _, l := range strings.Split(golden, "\n") {
		switch {
		case strings.HasPrefix(l, "# "):
			shape = strings.TrimPrefix(l, "# ")
		case l != "":
			fields = append(fields, shape+": "+l)
		}
	}
	return fields
}

// contractFill sets every exported field of v to a non-zero value,
// recursively, so each one appears in the JSON. Maps get a single "*" entry.
func contractFill(v reflect.Value) {
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.5)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		contractFill(v.Elem())
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 1, 1)
		contractFill(s.Index(0))
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		k := reflect.New(v.Type().Key()).Elem()
		if k.Kind() == reflect.String {
			k.SetString("*")
		} else {
			contractFill(k)
		}
		e := reflect.New(v.Type().Elem()).Elem()
		contractFill(e)
		m.SetMapIndex(k, e)
		v.Set(m)
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			v.Set(reflect.ValueOf(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				contractFill(v.Field(i))
			}
		}
	}
}

// contractPaths records the JSON type of every field path in v: "A.B" for a
// nested field, "A[]" for a list's elements, "[]" for a top-level list.
func contractPaths(v any, path string, out map[string]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			p := k
			if path != "" {
				p = path + "." + k
			}
			contractPaths(e, p, out)
		}
	case []any:
		for _, e := range x {
			contractPaths(e, path+"[]", out)
		}
	case string:
		out[path] = "string"
	case float64:
		out[path] = "number"
	case bool:
		out[path] = "bool"
	case nil:
		out[path] = "null"
	}
}
