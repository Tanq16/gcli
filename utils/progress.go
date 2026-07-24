package utils

import (
	"sync/atomic"
	"time"
)

// StartProgress runs a 1s ticker that renders a live progress line via render,
// which returns the current label and percent. The returned stop function halts
// the ticker and clears any line in flight; a tick already rendering must not
// print after the caller's final summary line, so stop waits for it to finish.
func StartProgress(render func() (label string, percent int)) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	var printed atomic.Bool
	go func() {
		defer close(finished)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		first := true
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !first {
					ClearPreviousLine()
				}
				first = false
				printed.Store(true)
				label, pct := render()
				PrintProgress(label, pct)
			}
		}
	}()
	return func() {
		close(done)
		<-finished
		if printed.Load() {
			ClearPreviousLine()
		}
	}
}
