// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"fmt"
	"sync"

	"github.com/triggermesh/brokers/pkg/common/fs"
)

type FakeCachedFileWatcher interface {
	fs.CachedFileWatcher
	SetContent(path string, content []byte) error
}

type watchedItem struct {
	content []byte
}

type fakeCachedFileWatcher struct {
	watchedFiles map[string]*watchedItem

	m sync.RWMutex
}

func NewCachedFileWatcher() FakeCachedFileWatcher {
	return &fakeCachedFileWatcher{
		watchedFiles: make(map[string]*watchedItem),
	}
}

func (ccw *fakeCachedFileWatcher) Start(_ context.Context) {}

func (ccw *fakeCachedFileWatcher) Add(path string, cb fs.CachedWatchCallback) error {
	ccw.m.Lock()
	defer ccw.m.Unlock()

	if _, ok := ccw.watchedFiles[path]; !ok {
		ccw.watchedFiles[path] = nil
	}

	return nil
}

func (ccw *fakeCachedFileWatcher) GetContent(path string) ([]byte, error) {
	ccw.m.RLock()
	defer ccw.m.RUnlock()

	watched, ok := ccw.watchedFiles[path]
	if !ok {
		return nil, fmt.Errorf("file %q is not being watched", path)
	}

	if watched != nil {
		return watched.content, nil
	}

	return nil, nil
}

func (ccw *fakeCachedFileWatcher) SetContent(path string, content []byte) error {
	ccw.m.Lock()
	defer ccw.m.Unlock()

	if _, ok := ccw.watchedFiles[path]; !ok {
		return fmt.Errorf("file %q is not being watched", path)
	}

	ccw.watchedFiles[path].content = content
	return nil
}
