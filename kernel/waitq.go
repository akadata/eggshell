// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"sync"
)

// Tasks blocked on a wait queue sleep on their own wake channel and are woken
// only by events on the objects they wait for, rather than by every event (see
// wakeAll).
//
// Each blocking call has its own sleeper, as a task can block from several
// goroutines at once (the kernel task used by kernel services).
//
// Wake ups do not require klock, so that they can be raised directly by
// goroutines not holding it (e.g. network stack callbacks): a sleeper is added
// to the wait queue before its condition is checked, and a wake up raised
// before it sleeps is kept in its channel.
//
// Until all objects have wait queues, wakeAll also wakes tasks blocked on wait
// queues, as conditions they check (e.g. pending signals) might change.

// sleeper represents a blocking call of a task on a wait queue.
type sleeper struct {
	t  *Task
	ch chan struct{}
}

var sleepers = sync.Pool{
	New: func() any {
		return &sleeper{ch: make(chan struct{}, 1)}
	},
}

// wake wakes the sleeper, it does not require klock.
func (w *sleeper) wake() {
	select {
	case w.ch <- struct{}{}:
	default:
	}
}

// sleep releases klock until the sleeper is woken.
func (w *sleeper) sleep() {
	klock.Unlock()
	<-w.ch
	klock.Lock()
}

// waitQueue represents the blocking calls waiting for an event on an object.
type waitQueue struct {
	mu       sync.Mutex
	sleepers []*sleeper
}

func (q *waitQueue) add(w *sleeper) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.sleepers = append(q.sleepers, w)
}

func (q *waitQueue) remove(w *sleeper) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, o := range q.sleepers {
		if o == w {
			last := len(q.sleepers) - 1
			q.sleepers[i] = q.sleepers[last]
			q.sleepers[last] = nil
			q.sleepers = q.sleepers[:last]
			return
		}
	}
}

// wake wakes the sleepers, which check again their wait condition, it does not
// require klock.
func (q *waitQueue) wake() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, w := range q.sleepers {
		w.wake()
	}
}

// sleepers on wait queues, klock must be held
var queued = map[*sleeper]struct{}{}

// enqueue adds a sleeper for the task to the argument wait queue, klock must be
// held.
func (t *Task) enqueue(q *waitQueue) *sleeper {
	w := sleepers.Get().(*sleeper)
	w.t = t

	// discard a wake up raised after its previous use
	select {
	case <-w.ch:
	default:
	}

	q.add(w)
	queued[w] = struct{}{}

	return w
}

// dequeue removes the sleeper from the argument wait queue, klock must be
// held.
func (t *Task) dequeue(q *waitQueue, w *sleeper) {
	delete(queued, w)
	q.remove(w)

	w.t = nil
	sleepers.Put(w)
}

// blockSelf is like block for conditions only changed by signals or the
// deadline (e.g. sleeps), which wake the task directly.
func (t *Task) blockSelf(ready func() bool, deadline int64, interruptible bool) Errno {
	var q waitQueue
	return t.blockOn(&q, ready, deadline, interruptible)
}

// wakeForSignal wakes the task, if blocked, to check its pending signals.
func (t *Task) wakeForSignal() {
	queuedTask := false

	for w := range queued {
		if w.t == t {
			w.wake()
			queuedTask = true
		}
	}

	if !queuedTask {
		kcond.Broadcast()
	}
}

// wakeQueued wakes all tasks blocked on wait queues.
func wakeQueued() {
	for w := range queued {
		w.wake()
	}
}
