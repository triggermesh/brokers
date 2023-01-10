package fs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

type PollerCallback func(content []byte)

type Poller interface {
	Add(path string, cb PollerCallback) error
	Start(ctx context.Context)
	GetContent(path string) ([]byte, error)
}

type pollFile struct {
	cbs            []PollerCallback
	cachedContents []byte
}

type poller struct {
	polledFiles map[string]*pollFile

	period time.Duration
	m      sync.RWMutex
	start  sync.Once
	logger *zap.SugaredLogger
}

func NewPoller(period time.Duration, logger *zap.SugaredLogger) (Poller, error) {
	return &poller{
		period:      period,
		logger:      logger,
		polledFiles: map[string]*pollFile{},
	}, nil
}

func (p *poller) GetContent(path string) ([]byte, error) {
	p.m.RLock()
	defer p.m.RUnlock()

	pf, ok := p.polledFiles[path]
	if !ok {
		return nil, fmt.Errorf("file %q is not being polled", path)
	}

	return pf.cachedContents, nil
}

func (p *poller) Add(path string, cb PollerCallback) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("error resolving to absolute path %q: %w", path, err)
	}

	if absPath != path {
		return fmt.Errorf("configuration path %q needs to be abstolute", path)
	}

	p.m.Lock()
	defer p.m.Unlock()

	p.logger.Infow("Adding file to poller", zap.String("file", path))
	if _, ok := p.polledFiles[path]; !ok {

		p.polledFiles[path] = &pollFile{cbs: []PollerCallback{cb}}
		return nil
	}

	pf := p.polledFiles[path]
	pf.cbs = append(pf.cbs, cb)
	p.polledFiles[path] = pf

	// force first polling
	p.poll()

	return nil
}

func (p *poller) Start(ctx context.Context) {
	p.start.Do(func() {

		ticker := time.NewTicker(p.period)
		// Do not block, exit on context done.
		go func() {
			for {

				p.poll()

				select {
				case <-ctx.Done():
					p.logger.Debug("Exiting file poller process")
					return
				case <-ticker.C:
					// file polling at the start of the loop
				}
			}
		}()
	})
}

func (p *poller) poll() {
	p.m.RLock()
	defer p.m.RUnlock()

	for file, pf := range p.polledFiles {
		b, err := os.ReadFile(file)
		if err != nil {
			p.logger.Errorw("cannot poll file", zap.String("filed", file), zap.Error(err))
		}

		if !bytes.Equal(pf.cachedContents, b) {
			p.logger.Infow("Existing", zap.String("contents", string(pf.cachedContents)))
			p.logger.Infow("New", zap.String("contents", string(b)))
			pf.cachedContents = b
			for _, cb := range pf.cbs {
				cb(b)
			}
		}
	}
}
