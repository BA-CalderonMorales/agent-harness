package state

import (
	"runtime"
	"testing"
)

func TestProjectSlugPreservesUnixNamesAndSupportsWindowsPaths(t *testing.T) {
	cases := map[string]string{
		"/home/me/project":  "home-me-project",
		"/home/me/project/": "home-me-project",
	}
	if runtime.GOOS == "windows" {
		cases[`C:\Users\me\project`] = "C-Users-me-project"
		cases[`D:\Users\me\project`] = "D-Users-me-project"
		cases[`\\server\share\project`] = "server-share-project"
	} else {
		cases["/home/me/project:archive"] = "home-me-project:archive"
		cases[`/home/me/project\archive`] = `home-me-project\archive`
	}
	for path, want := range cases {
		if got := projectSlug(path); got != want {
			t.Errorf("projectSlug(%q) = %q, want %q", path, got, want)
		}
	}
}
