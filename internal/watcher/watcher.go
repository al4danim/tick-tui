// Package watcher monitors a single file for external modifications and
// invokes a callback (debounced) whenever the file is written, created, or
// renamed in place.
//
// We watch the parent directory rather than the file itself because atomic
// writes (.tmp + rename) replace the inode, and a per-file watch would lose
// its target after the first such replacement.
package watcher

import (
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounce = 150 * time.Millisecond

// Watch starts watching `target`. onChange fires after a short debounce
// whenever the file changes. The returned stop function shuts down the
// watcher and joins the goroutine.
func Watch(target string, onChange func()) (stop func(), err error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(target)
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return nil, err
	}
	canon := filepath.Clean(target)

	done := make(chan struct{})
	go func() {
		var timer *time.Timer
		fire := func() {
			if onChange != nil {
				onChange()
			}
		}
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Clean(ev.Name) != canon {
					continue
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(debounce, fire)
			case <-w.Errors:
				// ignore — transient watcher errors shouldn't kill the loop
			case <-done:
				if timer != nil {
					timer.Stop()
				}
				_ = w.Close()
				return
			}
		}
	}()

	return func() { close(done) }, nil
}
