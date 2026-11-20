package main

import (
	"regexp"
	"runtime/debug"
)

// devVersion is what a build reports when neither the linker nor the Go
// toolchain stamped a version into it.
const devVersion = "dev"

// releaseVersion matches a clean release tag: v1.2.3, optionally with an
// alpha, beta, or rc suffix. Anything else (git describe distance, a
// pseudo-version, a dirty tree) is a development build.
var releaseVersion = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-(alpha|beta|rc)[0-9A-Za-z.]*)?$`)

func init() {
	info, ok := debug.ReadBuildInfo()
	Version = resolveVersion(Version, info, ok)
}

// resolveVersion picks the version a binary reports. The git tag is the
// only source of truth, and it arrives one of two ways: the linker stamps
// it (make build, release CI), or the Go toolchain records it as the main
// module version (go install ...@vX.Y.Z). With neither, the build is dev.
func resolveVersion(stamped string, info *debug.BuildInfo, ok bool) string {
	if stamped != "" && stamped != devVersion {
		return stamped
	}
	if ok && info != nil {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return devVersion
}

// buildType labels a version as a release or a development build.
func buildType(version string) string {
	if releaseVersion.MatchString(version) {
		return "release"
	}
	return "dev"
}
