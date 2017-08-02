// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package rendezvous

import (
	"sync"

	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// New allocates a new rendezvous meeting point for two coroutines.
func New() *Rendezvous {
	m := &sync.Mutex{}
	return &Rendezvous{
		lock: m,
		cond: sync.NewCond(m),
		turn: PartyA,
		data: nil,
		done: false,
	}
}

// Party represents one of two meeting parties for a rendezvous point.
type Party int

const (
	PartyA Party = 0
	PartyB Party = 1
)

// Rendezvous is a meeting point for two coroutines that ensures only one proceeds at a given time.
type Rendezvous struct {
	lock *sync.Mutex // locks to synchronize access to the turn.
	cond *sync.Cond  // condition variables to awaken parties as needed.
	turn Party       // the current party permitted through the rendezvous.
	data interface{} // optional data exchanged with the other party.
	done bool        // true when the rendezvous has become closed.
	err  error       // non-nil if an error was submitted at closing time.
}

// Let lets the other party to run a turn and then returns.
func (rz *Rendezvous) Let(me Party) (interface{}, bool, error) {
	rz.lock.Lock()
	defer rz.lock.Unlock()
	for rz.turn != me && !rz.done {
		rz.cond.Wait()
	}
	return rz.data, rz.done, rz.err
}

// Meet arrives at the meeting point and gives away the current turn.  The turn must be owned by the current party.
func (rz *Rendezvous) Meet(me Party, data interface{}) (interface{}, bool, error) {
	rz.lock.Lock()
	defer rz.lock.Unlock()

	// Give away the turn.
	contract.Assert(rz.turn == me)
	rz.data = data   // advertise our new data (if any).
	rz.flipTurn()    // let the other party run.
	rz.cond.Signal() // now signal to them, in case they are awaiting.

	// Wait until its our turn again.
	for rz.turn != me && !rz.done {
		rz.cond.Wait()
	}

	return rz.data, rz.done, rz.err
}

// Done signals to any waiters that the rendezvous is done.
func (rz *Rendezvous) Done(err error) {
	rz.lock.Lock()
	defer rz.lock.Unlock()
	rz.err = err
	rz.done = true
	rz.cond.Broadcast() // awaken any in case they are sleeping
}

// flipTurn lets the opposite party through the rendezvous point.
func (rz *Rendezvous) flipTurn() {
	if rz.turn == PartyA {
		rz.turn = PartyB
	} else {
		rz.turn = PartyA
	}
}
