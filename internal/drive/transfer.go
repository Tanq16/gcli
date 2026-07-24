package drive

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	u "github.com/tanq16/gcli/utils"
)

type task struct {
	relPath string
	bytes   int64
	run     func(ctx context.Context) error
}

// ItemError lets batch transfers report partial success (exit 6) instead of aborting on the first failure.
type ItemError struct {
	RelPath string
	Err     error
}

func (e ItemError) Error() string { return e.RelPath + ": " + e.Err.Error() }
func (e ItemError) Unwrap() error { return e.Err }

type TransferResult struct {
	Files   int
	Bytes   int64
	Skipped []string
	Errors  []ItemError
}

type ByteProgress struct {
	doneBytes  atomic.Int64
	doneFiles  atomic.Int64
	totalBytes int64
	totalFiles int64
}

func newByteProgress(totalFiles int, totalBytes int64) *ByteProgress {
	return &ByteProgress{totalBytes: totalBytes, totalFiles: int64(totalFiles)}
}

func (p *ByteProgress) writer() io.Writer { return byteCounter{p} }

type byteCounter struct{ p *ByteProgress }

func (w byteCounter) Write(b []byte) (int, error) {
	w.p.doneBytes.Add(int64(len(b)))
	return len(b), nil
}

// Exports report no size, so a fully-export batch falls back to file-count weighting.
func (p *ByteProgress) percent() int {
	switch {
	case p.totalBytes > 0:
		return int(p.doneBytes.Load() * 100 / p.totalBytes)
	case p.totalFiles > 0:
		return int(p.doneFiles.Load() * 100 / p.totalFiles)
	default:
		return 0
	}
}

func (p *ByteProgress) render(verb string) (string, int) {
	label := fmt.Sprintf("%s %d/%d files", verb, p.doneFiles.Load(), p.totalFiles)
	if p.totalBytes > 0 {
		label += fmt.Sprintf(" (%s / %s)", u.FormatSize(p.doneBytes.Load()), u.FormatSize(p.totalBytes))
	}
	return label, p.percent()
}

// Errors accumulate per-item instead of cancelling on the first failure, and the semaphore is acquired before spawning so goroutine count never exceeds workers.
func runPool(ctx context.Context, workers int, tasks []task, prog *ByteProgress) []ItemError {
	workers = max(1, workers)
	sem := make(chan struct{}, workers)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []ItemError
	)
	for _, t := range tasks {
		select {
		case <-ctx.Done():
			mu.Lock()
			errs = append(errs, ItemError{t.relPath, ctx.Err()})
			mu.Unlock()
			continue
		case sem <- struct{}{}:
		}
		wg.Go(func() {
			defer func() { <-sem }()
			if err := t.run(ctx); err != nil {
				mu.Lock()
				errs = append(errs, ItemError{t.relPath, err})
				mu.Unlock()
				return
			}
			prog.doneFiles.Add(1)
		})
	}
	wg.Wait()
	return errs
}

func runTasks(ctx context.Context, workers int, verb string, tasks []task, prog *ByteProgress) []ItemError {
	if len(tasks) == 0 {
		return nil
	}
	stop := u.StartProgress(func() (string, int) { return prog.render(verb) })
	errs := runPool(ctx, workers, tasks, prog)
	stop()
	return errs
}
