package record

import (
	"fmt"
	"hash/fnv"
)

const (
	maxCallIDLen    = 40
	callIDPrefixLen = 24
)

// boundID is the stored form of a tool-call id: short ids stay as they are,
// long ones keep a 24-byte prefix plus an FNV-1a hash of the whole id, so the
// record stays bounded however large a provider's id is. Both the call and
// its result go through it, so they still match.
func boundID(id string) string {
	if len(id) <= maxCallIDLen {
		return id
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return fmt.Sprintf("%s~%08x", id[:callIDPrefixLen], h.Sum32())
}
