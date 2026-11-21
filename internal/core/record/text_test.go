package record

import (
	"reflect"
	"testing"
)

func TestConstraintsKeepPeriodsInsideNames(t *testing.T) {
	cases := map[string][]string{
		"Never edit .env":                                   {"Never edit .env"},
		"Only edit config.yml":                              {"Only edit config.yml"},
		"Do not touch cmd/agent-harness/main.go.":           {"Do not touch cmd/agent-harness/main.go"},
		"Please start. Never edit a.go! Always run vet. ok": {"Never edit a.go", "Always run vet"},
	}
	for in, want := range cases {
		if got := constraintsIn(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
