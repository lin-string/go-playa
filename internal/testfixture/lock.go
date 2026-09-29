package testfixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

const fixtureLockStaleAfter = 35 * time.Minute

func withFixtureLock(ctx context.Context, lockPath string, fn func() error) (err error) {
	for {
		mkdirErr := os.Mkdir(lockPath, 0o700)
		if mkdirErr == nil {
			defer func() {
				if removeErr := os.Remove(lockPath); err == nil && removeErr != nil {
					err = fmt.Errorf("release fixture fetch lock %q: %w", lockPath, removeErr)
				}
			}()
			return fn()
		}
		if !errors.Is(mkdirErr, os.ErrExist) {
			return fmt.Errorf("acquire fixture fetch lock %q: %w", lockPath, mkdirErr)
		}
		info, statErr := os.Stat(lockPath)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("inspect fixture fetch lock %q: %w", lockPath, statErr)
		}
		if statErr == nil && time.Since(info.ModTime()) > fixtureLockStaleAfter {
			removeErr := os.Remove(lockPath)
			if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("reclaim stale fixture fetch lock %q: %w", lockPath, removeErr)
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("wait for fixture fetch lock %q: %w", lockPath, ctx.Err())
		case <-timer.C:
		}
	}
}
