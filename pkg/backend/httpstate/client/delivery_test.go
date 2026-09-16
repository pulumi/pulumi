// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestCompleteDeliveryTransitionRequest(t *testing.T) {
	t.Parallel()
	var body CompleteDeliveryTransitionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/preview/stacks/acme/shop/pipeline/delivery/transitions/transition%2F1/complete", r.URL.EscapedPath())
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	api := NewClient(server.URL, "token", true, diag.DefaultSink(io.Discard, io.Discard,
		diag.FormatOptions{Color: colors.Never}))
	err := api.CompleteDeliveryTransition(t.Context(), StackIdentifier{Owner: "acme", Project: "shop", Stack: tokens.MustParseStackName("pipeline")},
		"transition/1", CompleteDeliveryTransitionRequest{AttemptID: "attempt", WorkflowRunID: "run", Phase: "succeeded",
			Outputs: json.RawMessage(`{"digest":{"4dabf18193072939515e22adb298388d":"1b47061264138c4ac30d75fd1eb44270"}}`)})
	require.NoError(t, err)
	assert.Equal(t, "attempt", body.AttemptID)
	assert.Equal(t, "run", body.WorkflowRunID)
	assert.JSONEq(t, `{"digest":{"4dabf18193072939515e22adb298388d":"1b47061264138c4ac30d75fd1eb44270"}}`, string(body.Outputs))
}

func TestCompleteDeliveryCandidatePreview(t *testing.T) {
	t.Parallel()
	var body DeliveryCandidatePreviewRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/preview/stacks/acme/shop/pipeline/delivery/candidates/candidate%2F1/preview", r.URL.EscapedPath())
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	api := NewClient(server.URL, "token", true, diag.DefaultSink(io.Discard, io.Discard,
		diag.FormatOptions{Color: colors.Never}))
	err := api.CompleteDeliveryCandidatePreview(t.Context(), StackIdentifier{Owner: "acme", Project: "shop",
		Stack: tokens.MustParseStackName("pipeline")}, "candidate/1", DeliveryCandidatePreviewRequest{
		ShapeVersion: 2, ReleaseID: "release", ChangeRequestID: "cr", RevisionNumber: 3,
		WorkflowRunID: "run", Status: "succeeded", Stacks: []DeliveryCandidatePreviewStack{},
		Plan: json.RawMessage(`{"resourcePlans":{}}`), Events: json.RawMessage(`[]`),
	})
	require.NoError(t, err)
	assert.Equal(t, "run", body.WorkflowRunID)
	assert.JSONEq(t, `{"resourcePlans":{}}`, string(body.Plan))
}

func TestListDeliveryReleasesContinuationToken(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/preview/stacks/acme/shop/pipeline/delivery/releases", r.URL.Path)
		assert.Equal(t, "next/page", r.URL.Query().Get("continuationToken"))
		_, err := io.WriteString(w, `{"releases":[]}`)
		require.NoError(t, err)
	}))
	defer server.Close()

	api := NewClient(server.URL, "token", true, diag.DefaultSink(io.Discard, io.Discard,
		diag.FormatOptions{Color: colors.Never}))
	_, err := api.ListDeliveryReleases(t.Context(), StackIdentifier{
		Owner: "acme", Project: "shop", Stack: tokens.MustParseStackName("pipeline"),
	}, "next/page")
	require.NoError(t, err)
}
