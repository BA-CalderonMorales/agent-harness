package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	module := func(v string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Version: v}}
	}
	cases := []struct {
		name    string
		stamped string
		info    *debug.BuildInfo
		ok      bool
		want    string
	}{
		{"linker stamp wins over module version", "v0.4.2", module("v0.4.1"), true, "v0.4.2"},
		{"go install records the module version", devVersion, module("v0.4.2"), true, "v0.4.2"},
		{"local checkout without vcs info is dev", devVersion, module("(devel)"), true, devVersion},
		{"empty module version is dev", devVersion, module(""), true, devVersion},
		{"no build info is dev", devVersion, nil, false, devVersion},
		{"empty stamp falls through", "", module("v0.4.2"), true, "v0.4.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.stamped, tc.info, tc.ok); got != tc.want {
				t.Fatalf("resolveVersion(%q) = %q, want %q", tc.stamped, got, tc.want)
			}
		})
	}
}

func TestBuildType(t *testing.T) {
	cases := map[string]string{
		"v0.4.2":                               "release",
		"0.4.2":                                "release",
		"v1.0.0-rc.1":                          "release",
		"v1.0.0-beta2":                         "release",
		devVersion:                             "dev",
		"v0.4.1-3-gabc1234":                    "dev",
		"v0.4.1-dirty":                         "dev",
		"v0.4.2-0.20261119000000-f282c9cc48de": "dev",
		"f282c9c":                              "dev",
	}
	for version, want := range cases {
		if got := buildType(version); got != want {
			t.Errorf("buildType(%q) = %q, want %q", version, got, want)
		}
	}
}
