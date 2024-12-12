package utils

import (
	"sync"
	"sync/atomic"
)

type Event struct {
	fired atomic.Int32
	c     chan struct{}
	o     sync.Once
}

func (e *Event) Fire() bool {
	ret := false
	e.o.Do(func() {
		e.fired.Store(1)
		close(e.c)
		ret = true
	})
	return ret
}

func (e *Event) Fired() <-chan struct{} {
	return e.c
}

func (e *Event) HasFired() bool {
	return e.fired.Load() > 0
}

func NewEvent() *Event {
	return &Event{c: make(chan struct{})}
}
