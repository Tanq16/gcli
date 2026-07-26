package utils

import (
	"sync/atomic"
	"time"
)

// stop waits for an in-flight tick to finish, so a late render can't print after the caller's final summary line.
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
