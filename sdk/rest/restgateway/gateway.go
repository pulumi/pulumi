// Copyright 2016-2026, Pulumi Corporation.
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

package restgateway

import (
	"fmt"
	"net/http"
	"sync"
)

// Gateway is the REST HTTP gateway for the Pulumi engine.
type Gateway struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewGateway creates a new Gateway.
func NewGateway() *Gateway {
	return &Gateway{
		sessions: make(map[string]*Session),
	}
}

// Handler returns the HTTP handler for the gateway.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /sessions", g.handleCreateSession)
	mux.HandleFunc("POST /sessions/{id}/resources", g.handleRegisterResource)
	mux.HandleFunc("POST /sessions/{id}/invoke", g.handleInvoke)
	mux.HandleFunc("POST /sessions/{id}/logs", g.handleLog)
	mux.HandleFunc("DELETE /sessions/{id}", g.handleDeleteSession)
	return mux
}

// GetSession retrieves a session by ID, returning an error if not found.
func (g *Gateway) GetSession(id string) (*Session, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	sess, ok := g.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return sess, nil
}

// AddSession stores a session in the gateway.
func (g *Gateway) AddSession(sess *Session) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sessions[sess.ID] = sess
}

// RemoveSession removes a session from the gateway.
func (g *Gateway) RemoveSession(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.sessions, id)
}
