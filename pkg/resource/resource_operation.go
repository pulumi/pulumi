// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

// OperationState is the state of operations issued by the engine.
type OperationState string

const (
	// OperationStateCreating is the state of resources that are being created.
	OperationStateCreating OperationState = "creating"
	// OperationStateUpdating is the state of resources that are being updated.
	OperationStateUpdating OperationState = "updating"
	// OperationStateDeleting is the state of resources that are being deleted.
	OperationStateDeleting OperationState = "deleting"
	// OperationStateReading is the state of resources that are being read.
	OperationStateReading OperationState = "reading"
)

// Operation represents an operation that the engine has initiated but has not yet completed. It is
// essentially just a tuple of a resource and a string identifying the operation.
type Operation struct {
	Resource  *State
	Operation OperationState
}

// NewOperation constructs a new Operation from a state and an operation name.
func NewOperation(state *State, op OperationState) Operation {
	return Operation{state, op}
}
