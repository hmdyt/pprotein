package collect

import (
	"bytes"
	"fmt"
	"io"

	"github.com/hmdyt/pprotein/internal/storage"
)

const cacheTypeKey = "cache"

type (
	Processor interface {
		Process(snapshot *Snapshot) (io.ReadCloser, error)
		Cacheable() bool
	}

	// Versioned is implemented by processors whose output depends on
	// something besides the snapshot (e.g. a config file). The version is
	// folded into the cache key so that changing it invalidates the cache.
	Versioned interface {
		Version() (string, error)
	}

	cachedProcessor struct {
		internal Processor
		store    storage.Storage
	}
)

func newCachedProcessor(internal Processor, store storage.Storage) Processor {
	return &cachedProcessor{internal, store}
}

func (p *cachedProcessor) cacheKey(snapshot *Snapshot) (string, error) {
	v, ok := p.internal.(Versioned)
	if !ok {
		return snapshot.ID, nil
	}
	version, err := v.Version()
	if err != nil {
		return "", fmt.Errorf("failed to get processor version: %w", err)
	}
	return snapshot.ID + "@" + version, nil
}

func (p *cachedProcessor) Process(snapshot *Snapshot) (io.ReadCloser, error) {
	key, err := p.cacheKey(snapshot)
	if err != nil {
		return nil, err
	}
	if ok, err := p.store.Exists(cacheTypeKey, key); err != nil {
		return nil, fmt.Errorf("failed to check cache status: %w", err)
	} else if ok {
		return p.serveCached(key)
	}
	return p.serveGenerated(snapshot, key)
}
func (p *cachedProcessor) serveCached(key string) (io.ReadCloser, error) {
	cache, err := p.store.Get(cacheTypeKey, key)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache: %w", err)
	}
	return io.NopCloser(bytes.NewBuffer(cache)), nil
}
func (p *cachedProcessor) serveGenerated(snapshot *Snapshot, key string) (io.ReadCloser, error) {
	r, err := p.internal.Process(snapshot)
	if err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}

	if !p.internal.Cacheable() {
		return r, nil
	}

	cacheContent, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	if err := p.store.Put(cacheTypeKey, key, cacheContent); err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}
	return p.serveCached(key)
}

func (p *cachedProcessor) Cacheable() bool {
	return false
}
