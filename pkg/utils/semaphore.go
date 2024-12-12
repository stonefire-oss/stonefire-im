package utils

import "sync/atomic"

type AtomicSemaphore struct {
	n    atomic.Int64
	wait chan struct{}
}

func (q *AtomicSemaphore) Acquire() {
	if q.n.Add(-1) < 0 {
		// We ran out of quota.  Block until a release happens.
		<-q.wait
	}
}

func (q *AtomicSemaphore) Release() {
	// N.B. the "<= 0" check below should allow for this to work with multiple
	// concurrent calls to acquire, but also note that with synchronous calls to
	// acquire, as our system does, n will never be less than -1.  There are
	// fairness issues (queuing) to consider if this was to be generalized.
	if q.n.Add(1) <= 0 {
		// An acquire was waiting on us.  Unblock it.
		q.wait <- struct{}{}
	}
}

func NewHandlerQuota(n uint32) *AtomicSemaphore {
	a := &AtomicSemaphore{wait: make(chan struct{}, 1)}
	a.n.Store(int64(n))
	return a
}
