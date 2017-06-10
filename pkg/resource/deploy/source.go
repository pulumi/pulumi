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

package deploy

import (
	"io"

	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
)

// A Source can generate a new set of resources that the planner will process accordingly.
type Source interface {
	io.Closer
	// Info returns a serializable payload that can be used to stamp snapshots for future reconciliation.
	Info() interface{}
	// Iterate begins iterating the source.  Error is non-nil upon failure; otherwise, a valid iterator is returned.
	Iterate() (SourceIterator, error)
}

// A SourceIterator enumerates the list of resources that a source has to offer.
type SourceIterator interface {
	io.Closer
	// Next returns the next resource object plus a token context (usually where it was allocated); the token is used in
	// the production of the ensuing resource's URN.  If something went wrong, error is non-nil.  If both error and the
	// resource are nil, then the iterator has completed its job and no subsequent calls to next should be made.
	Next() (*resource.Object, tokens.Module, error)
}
