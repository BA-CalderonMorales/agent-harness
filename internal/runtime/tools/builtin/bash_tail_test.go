package builtin

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestBashPreservesUnterminatedOutput(t *testing.T) {
	for _, command := range []string{"printf tail", "printf tail >&2", "printf 'first\\ntail'", "printf tail; exit 7", "printf tail; sleep 5"} {
		t.Run(command, func(t *testing.T) {
			var mu sync.Mutex
			var progress []string
			result, err := runBashCommand(context.Background(), command, 100, func(data any) {
				mu.Lock()
				defer mu.Unlock()
				progress = append(progress, data.(string))
			})
			if err != nil || !strings.Contains(result.Data.(string), "tail") {
				t.Fatalf("result=%v err=%v", result, err)
			}
			mu.Lock()
			defer mu.Unlock()
			count := 0
			for _, line := range progress {
				if line == "tail" {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("tail emitted %d times in %v", count, progress)
			}
		})
	}
}
