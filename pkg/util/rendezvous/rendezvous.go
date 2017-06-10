// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rendezvous

import (
	"sync"

	"github.com/pulumi/lumi/pkg/util/contract"
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

// Meet arrives at the rendezvous point.  It awaits the other party's arrival and returns whatever data they supply, if
// any.  If the bool it returns is false, that means the rendezvous point has been closed and the party should quit.
func (rz *Rendezvous) Meet(me Party, data interface{}) (interface{}, bool, error) {
	rz.lock.Lock()
	defer rz.lock.Unlock()

	// If it's our turn already, it is coming to an end, and we must give it away.
	if rz.turn == me {
		rz.data = data
		if rz.turn == PartyA {
			rz.turn = PartyB
		} else {
			rz.turn = PartyA
		}
		rz.cond.Signal()
	} else {
		contract.Assert(data == nil)
	}

	// Until it's our turn, we must wait.
	for rz.turn != me && !rz.done {
		rz.cond.Wait()
	}

	return rz.data, !rz.done, rz.err
}

// Done signals to any waiters that the rendezvous is done.
func (rz *Rendezvous) Done(err error) {
	rz.lock.Lock()
	defer rz.lock.Unlock()
	rz.err = err
	rz.done = true
	rz.cond.Broadcast() // awaken any in case they are sleeping
}
