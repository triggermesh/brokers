// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"fmt"
	"sync"

	"github.com/triggermesh/brokers/pkg/common/fs"
)

type FakeFileWatcher interface {
	fs.FileWatcher
	DoCallback(path string) error
}

type fakeFileWatcher struct {
	watchedFiles map[string][]fs.WatchCallback

	m sync.RWMutex
}

func NewFileWatcher() FakeFileWatcher {
	return &fakeFileWatcher{
		watchedFiles: make(map[string][]fs.WatchCallback),
	}
}

func (cw *fakeFileWatcher) Start(_ context.Context) {}

func (cw *fakeFileWatcher) Add(path string, cb fs.WatchCallback) error {
	cw.m.Lock()
	defer cw.m.Unlock()

	if _, ok := cw.watchedFiles[path]; !ok {
		cw.watchedFiles[path] = []fs.WatchCallback{}
	}
	cw.watchedFiles[path] = append(cw.watchedFiles[path], cb)

	return nil
}

func (cw *fakeFileWatcher) DoCallback(path string) error {
	cw.m.RLock()
	defer cw.m.RUnlock()

	cbs, ok := cw.watchedFiles[path]
	if !ok {
		return fmt.Errorf("path %q is not being watched", path)
	}

	for _, cb := range cbs {
		cb()
	}
	return nil
}
