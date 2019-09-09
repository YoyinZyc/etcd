package traceutil

import (
	"time"
)

type Trace struct {
	operation string
	startTime time.Time
	steps     []step
}

type step struct {
	time time.Time
	msg  string
}

func New(op string) *Trace {
	return &Trace{operation: op, startTime: time.Now()}
}

func (t *Trace) AddStep(msg string) {
	t.steps = append(t.steps, step{time: time.Now(), msg: msg})
}
