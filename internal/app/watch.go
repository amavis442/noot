package app

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch keeps the index in sync until ctx is cancelled. Events are debounced so
// an editor's save-rename-write dance results in a single sync.
func (s *Service) Watch(ctx context.Context, debounce time.Duration, logf func(string, ...any)) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	addTree := func(root string) {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if !s.Vault.ShouldWatchDir(p) {
				return filepath.SkipDir
			}
			if err := w.Add(p); err != nil {
				log.Printf("watch %s: %v", p, err)
			}
			return nil
		})
	}
	addTree(s.Vault.Root)

	sync := func() {
		st, err := s.Sync(ctx, false)
		if err != nil {
			logf("sync failed: %v", err)
			return
		}
		for _, e := range st.Errors {
			logf("warning: %v", e)
		}
		if st.Updated > 0 || st.Deleted > 0 {
			logf("%s", st)
		}
	}
	sync()

	var timer *time.Timer
	fire := make(chan struct{}, 1)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-w.Errors:
			logf("watcher: %v", err)
		case ev := <-w.Events:
			if ev.Has(fsnotify.Create) {
				if fi, err := statDir(ev.Name); err == nil && fi && s.Vault.ShouldWatchDir(ev.Name) {
					addTree(ev.Name)
				}
			}
			if timer == nil {
				timer = time.AfterFunc(debounce, func() { fire <- struct{}{} })
			} else {
				timer.Reset(debounce)
			}
		case <-fire:
			sync()
		}
	}
}

func statDir(p string) (bool, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}
