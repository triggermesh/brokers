// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// WatchCallback is called when a watched file
// is updated.
type WatchCallback func()

// FileWatcher object tracks changes to files.
type FileWatcher interface {
	Add(path string, cb WatchCallback) error
	Start(ctx context.Context)
}

type fileWatcher struct {
	watcher      *fsnotify.Watcher
	watchedFiles map[string][]WatchCallback

	m      sync.RWMutex
	start  sync.Once
	logger *zap.SugaredLogger
}

// NewWatcher creates a new FileWatcher object that register files
// and calls back when they change.
func NewWatcher(logger *zap.SugaredLogger) (FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &fileWatcher{
		watcher:      watcher,
		watchedFiles: make(map[string][]WatchCallback),
		logger:       logger,
	}, nil
}

// Add path/callback tuple to the  FileWatcher.
func (cw *fileWatcher) Add(path string, cb WatchCallback) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("error resolving to absoluthe path %q: %w", path, err)
	}

	if absPath != path {
		return fmt.Errorf("configuration path %q needs to be abstolute", path)
	}

	cw.m.Lock()
	defer cw.m.Unlock()

	cw.logger.Infow("Adding file to watch", zap.String("file", path))
	if _, ok := cw.watchedFiles[path]; !ok {
		if err := cw.watcher.Add(path); err != nil {
			return err
		}
		cw.watchedFiles[path] = []WatchCallback{cb}
		return nil
	}

	cw.watchedFiles[path] = append(cw.watchedFiles[path], cb)
	return nil
}

// Start the FileWatcher process.
func (cw *fileWatcher) Start(ctx context.Context) {
	cw.start.Do(func() {
		// Do not block, exit on context done.
		go func() {
			defer cw.watcher.Close()
			for {
				select {
				case e, ok := <-cw.watcher.Events:
					if !ok {
						// watcher event channel finished
						return
					}

					if e.Op&fsnotify.Remove == fsnotify.Remove {
						if fileExist(e.Name) {
							if err := cw.watcher.Add(e.Name); err != nil {
								cw.logger.Errorw(
									fmt.Sprintf("could not add the path %q back to the watcher", e.Name),
									zap.Error(err))
							}
						}
						continue
					}

					cw.m.RLock()
					cbs, ok := cw.watchedFiles[e.Name]
					if !ok {
						cw.logger.Warnw("Received a notification for a non watched file", zap.String("file", e.Name))
					}

					for _, cb := range cbs {
						cb()
					}
					cw.m.RUnlock()

				case err, ok := <-cw.watcher.Errors:
					if !ok {
						// watcher error channel finished
						return
					}
					cw.logger.Errorw("Error watching files", zap.Error(err))

				case <-ctx.Done():
					cw.logger.Debug("Exiting file watcher process")
					return
				}
			}
		}()
	})
}

func fileExist(file string) bool {
	_, err := os.Stat(file)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return true
}
