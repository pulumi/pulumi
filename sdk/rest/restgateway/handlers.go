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
	"encoding/json"
	"log"
	"net/http"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, ErrorResponse{Error: msg})
}

// handleCreateSession handles POST /sessions.
func (g *Gateway) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ProjectName == "" || req.Stack == "" {
		WriteError(w, http.StatusBadRequest, "projectName and stack are required")
		return
	}

	sess, err := NewSession(r.Context(), req.ProjectName, req.Stack, req.Preview)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to create session: "+err.Error())
		return
	}

	g.AddSession(sess)

	WriteJSON(w, http.StatusCreated, CreateSessionResponse{
		ID:       sess.ID,
		StackURN: sess.StackURN,
	})
}

// handleRegisterResource handles POST /sessions/{id}/resources.
func (g *Gateway) handleRegisterResource(w http.ResponseWriter, r *http.Request) {
	sess, err := g.GetSession(r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	var req RegisterResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Type == "" || req.Name == "" {
		WriteError(w, http.StatusBadRequest, "type and name are required")
		return
	}

	// Convert properties to protobuf.
	props, err := JSONToStruct(req.Properties)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid properties: "+err.Error())
		return
	}

	// Default parent to the stack URN if not provided.
	parent := req.Parent
	if parent == "" {
		parent = sess.StackURN
	}

	// Build the gRPC request.
	grpcReq := &pulumirpc.RegisterResourceRequest{
		Type:                    req.Type,
		Name:                    req.Name,
		Custom:                  req.Custom,
		Parent:                  parent,
		Object:                  props,
		Dependencies:            req.Dependencies,
		Provider:                req.Provider,
		Version:                 req.Version,
		ImportId:                req.ImportID,
		IgnoreChanges:           req.IgnoreChanges,
		AdditionalSecretOutputs: req.AdditionalSecretOutputs,
		ReplaceOnChanges:        req.ReplaceOnChanges,
		PluginDownloadURL:       req.PluginDownloadURL,
		HideDiffs:               req.HideDiffs,
		ReplaceWith:             req.ReplaceWith,
		AcceptSecrets:           true,
		AcceptResources:         true,
		SupportsResultReporting: true,
	}
	if req.Protect != nil {
		grpcReq.Protect = req.Protect
	}
	if req.RetainOnDelete != nil {
		grpcReq.RetainOnDelete = req.RetainOnDelete
	}
	if req.DeleteBeforeReplace != nil {
		grpcReq.DeleteBeforeReplace = *req.DeleteBeforeReplace
	}

	resp, err := sess.Monitor.RegisterResource(r.Context(), grpcReq)
	if err != nil {
		log.Printf("RegisterResource error: %v", err)
		WriteError(w, http.StatusInternalServerError, "register resource failed: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, RegisterResourceResponse{
		URN:        resp.Urn,
		ID:         resp.Id,
		Properties: StructToJSON(resp.Object),
		Stable:     resp.Stable,
	})
}

// handleInvoke handles POST /sessions/{id}/invoke.
func (g *Gateway) handleInvoke(w http.ResponseWriter, r *http.Request) {
	sess, err := g.GetSession(r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	var req InvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Token == "" {
		WriteError(w, http.StatusBadRequest, "token is required")
		return
	}

	args, err := JSONToStruct(req.Args)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid args: "+err.Error())
		return
	}

	grpcReq := &pulumirpc.ResourceInvokeRequest{
		Tok:               req.Token,
		Args:              args,
		Provider:          req.Provider,
		Version:           req.Version,
		PluginDownloadURL: req.PluginDownloadURL,
		AcceptResources:   true,
	}

	resp, err := sess.Monitor.Invoke(r.Context(), grpcReq)
	if err != nil {
		log.Printf("Invoke error: %v", err)
		WriteError(w, http.StatusInternalServerError, "invoke failed: "+err.Error())
		return
	}

	var failures []CheckFailure
	for _, f := range resp.Failures {
		failures = append(failures, CheckFailure{
			Property: f.Property,
			Reason:   f.Reason,
		})
	}

	WriteJSON(w, http.StatusOK, InvokeResponse{
		Return:   StructToJSON(resp.Return),
		Failures: failures,
	})
}

// handleLog handles POST /sessions/{id}/logs.
func (g *Gateway) handleLog(w http.ResponseWriter, r *http.Request) {
	sess, err := g.GetSession(r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	var req LogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Message == "" {
		WriteError(w, http.StatusBadRequest, "message is required")
		return
	}

	if sess.Engine == nil {
		WriteError(w, http.StatusServiceUnavailable, "engine service not available")
		return
	}

	grpcReq := &pulumirpc.LogRequest{
		Severity: SeverityToProto(req.Severity),
		Message:  req.Message,
		Urn:      req.URN,
	}

	if _, err := sess.Engine.Log(r.Context(), grpcReq); err != nil {
		log.Printf("Log error: %v", err)
		WriteError(w, http.StatusInternalServerError, "log failed: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, struct{}{})
}

// handleDeleteSession handles DELETE /sessions/{id}.
func (g *Gateway) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := g.GetSession(id)
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	// Parse optional exports from body.
	var req DeleteSessionRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}

	if err := sess.Close(r.Context(), req.Exports); err != nil {
		log.Printf("Session close error: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to close session: "+err.Error())
		return
	}

	g.RemoveSession(id)

	WriteJSON(w, http.StatusOK, struct{ Status string }{"shutdown_complete"})
}
