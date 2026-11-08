package mcpserver

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/glnreddy421/klew/v2/internal/model"
)

const (
	maxCachedInvestigations = 8
	investigationTTL        = 30 * time.Minute
)

type cachedInvestigation struct {
	state   model.InvestigationState
	created time.Time
}

// investigationCache keeps recent investigation states so detail calls page
// through them without collecting from the cluster again.
type investigationCache struct {
	mu    sync.Mutex
	max   int
	order []string
	items map[string]cachedInvestigation
	now   func() time.Time
}

func newInvestigationCache(max int) *investigationCache {
	return &investigationCache{max: max, items: map[string]cachedInvestigation{}, now: time.Now}
}

func (c *investigationCache) put(st model.InvestigationState) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	id := "inv_" + hex.EncodeToString(b)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropExpired()
	c.items[id] = cachedInvestigation{state: st, created: c.now()}
	c.order = append(c.order, id)
	for len(c.order) > c.max {
		delete(c.items, c.order[0])
		c.order = c.order[1:]
	}
	return id
}

func (c *investigationCache) get(id string) (model.InvestigationState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropExpired()
	it, ok := c.items[id]
	if !ok {
		return model.InvestigationState{}, false
	}
	return it.state, true
}

// dropExpired frees expired states, which hold raw logs and events. Entries
// are in creation order, so it stops at the first live one. Callers hold mu.
func (c *investigationCache) dropExpired() {
	for len(c.order) > 0 && c.now().Sub(c.items[c.order[0]].created) > investigationTTL {
		delete(c.items, c.order[0])
		c.order = c.order[1:]
	}
}
