package ui

import (
	"fmt"
	"sync"
	"time"
)

// Handler manages the terminal-native UI feedback.
type Handler struct {
	mu         sync.Mutex
	isSpinning bool
	stopChan   chan struct{}
}

// NewHandler creates a new UI handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Status prints a status message with the mandated indicators.
// Indicators: ◆ (start), → (action), ✓ (success), ✗ (error), ? (input)
func (h *Handler) Status(indicator, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Printf("%s %s\n", indicator, message)
}

// SpinnerStart starts a Kaomoji-style spinner.
func (h *Handler) SpinnerStart(label string) {
	h.mu.Lock()
	if h.isSpinning {
		h.mu.Unlock()
		return
	}
	h.isSpinning = true
	h.stopChan = make(chan struct{})
	h.mu.Unlock()

	frames := []string{"┌( >_<)┘", "└( >_<)┐"}
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-ticker.C:
				fmt.Printf("\r  %s  %s...", frames[i%len(frames)], label)
				i++
			case <-h.stopChan:
				fmt.Print("\r\033[K") // Clear line
				return
			}
		}
	}()
}

// SpinnerStop stops the active spinner.
func (h *Handler) SpinnerStop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.isSpinning {
		close(h.stopChan)
		h.isSpinning = false
	}
}
