package nex

import (
	"sync/atomic"
	"time"
)

// nexUniqueIDCounter is seeded from the current time so IDs stay unique
// across server restarts too, not just within a single run.
var nexUniqueIDCounter = func() *atomic.Uint64 {
	var counter atomic.Uint64
	counter.Store(uint64(time.Now().UnixNano()))
	return &counter
}()

func GenerateNEXUniqueID() uint64 {
	return nexUniqueIDCounter.Add(1)
}
