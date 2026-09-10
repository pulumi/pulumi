// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type DeliveryPipelineSnapshot struct {
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	ShapeVersion    int                     `json:"shapeVersion"`
	Runtime         any                     `json:"runtime,omitempty"`
	Commit          string                  `json:"commit,omitempty"`
	NewestReleaseID string                  `json:"newestReleaseId,omitempty"`
	ActionRequired  bool                    `json:"actionRequired"`
	Stages          []DeliveryStageSnapshot `json:"stages"`
}

type DeliveryStageSnapshot struct {
	Name           string           `json:"name"`
	State          string           `json:"state"`
	Paused         bool             `json:"paused"`
	PauseReason    string           `json:"pauseReason,omitempty"`
	Version        int              `json:"version"`
	Release        any              `json:"release,omitempty"`
	UpstreamStages []string         `json:"upstreamStages"`
	Members        []map[string]any `json:"members"`
	Rule           any              `json:"rule,omitempty"`
	Desired        *struct {
		ReleaseID  string `json:"releaseId"`
		Generation int    `json:"generation"`
	} `json:"desired,omitempty"`
}

type DeliveryRelease struct {
	ID                string         `json:"id"`
	PreviousReleaseID string         `json:"previousReleaseId,omitempty"`
	Parts             map[string]any `json:"parts"`
	Artifacts         []any          `json:"artifacts"`
	Journey           []any          `json:"journey"`
	CreatedAt         string         `json:"createdAt"`
}

type ListDeliveryReleasesResponse struct {
	Releases          []DeliveryRelease `json:"releases"`
	ContinuationToken string            `json:"continuationToken,omitempty"`
}
type ListDeliveryPassesResponse struct {
	Passes            []map[string]any `json:"passes"`
	ContinuationToken string           `json:"continuationToken,omitempty"`
}
type ListDeliveryEventsResponse struct {
	Events            []map[string]any `json:"events"`
	ContinuationToken string           `json:"continuationToken,omitempty"`
}
type DeliveryJobLogsResponse struct {
	Status string           `json:"status"`
	Lines  []map[string]any `json:"lines"`
}

type DeliveryStageActionRequest struct {
	Version   int    `json:"version"`
	ReleaseID string `json:"releaseId,omitempty"`
	Reason    string `json:"reason,omitempty"`
	RuleName  string `json:"ruleName,omitempty"`
}

type CompleteDeliveryTransitionRequest struct {
	AttemptID           string          `json:"attemptId"`
	WorkflowRunID       string          `json:"workflowRunId"`
	Phase               string          `json:"phase"`
	Outputs             json.RawMessage `json:"outputs,omitempty"`
	Message             string          `json:"message,omitempty"`
	EnvironmentRevision string          `json:"environmentRevision,omitempty"`
}

func deliveryPath(stack StackIdentifier, suffix string) string {
	return fmt.Sprintf("/api/preview/stacks/%s/%s/%s/delivery%s", url.PathEscape(stack.Owner),
		url.PathEscape(stack.Project), url.PathEscape(stack.Stack.String()), suffix)
}

func (pc *Client) GetDeliveryPipeline(ctx context.Context, stack StackIdentifier) (DeliveryPipelineSnapshot, error) {
	var response DeliveryPipelineSnapshot
	err := pc.restCall(ctx, "GET", deliveryPath(stack, "/pipeline"), nil, nil, &response)
	return response, err
}

func (pc *Client) ListDeliveryReleases(ctx context.Context, stack StackIdentifier,
	continuationToken string,
) (ListDeliveryReleasesResponse, error) {
	var response ListDeliveryReleasesResponse
	path := deliveryPath(stack, "/releases")
	if continuationToken != "" {
		path += "?continuationToken=" + url.QueryEscape(continuationToken)
	}
	err := pc.restCall(ctx, "GET", path, nil, nil, &response)
	return response, err
}

func (pc *Client) GetDeliveryRelease(ctx context.Context, stack StackIdentifier, release string) (DeliveryRelease, error) {
	var response DeliveryRelease
	err := pc.restCall(ctx, "GET", deliveryPath(stack, "/releases/"+url.PathEscape(release)), nil, nil, &response)
	return response, err
}

func (pc *Client) ListDeliveryPasses(ctx context.Context, stack StackIdentifier,
	continuationToken string,
) (ListDeliveryPassesResponse, error) {
	var response ListDeliveryPassesResponse
	path := deliveryPath(stack, "/passes")
	if continuationToken != "" {
		path += "?continuationToken=" + url.QueryEscape(continuationToken)
	}
	err := pc.restCall(ctx, "GET", path, nil, nil, &response)
	return response, err
}

func (pc *Client) ListDeliveryEvents(ctx context.Context, stack StackIdentifier,
	continuationToken string,
) (ListDeliveryEventsResponse, error) {
	var response ListDeliveryEventsResponse
	path := deliveryPath(stack, "/events")
	if continuationToken != "" {
		path += "?continuationToken=" + url.QueryEscape(continuationToken)
	}
	err := pc.restCall(ctx, "GET", path, nil, nil, &response)
	return response, err
}

func (pc *Client) DeliveryStageAction(ctx context.Context, stack StackIdentifier, stage, action string,
	request DeliveryStageActionRequest,
) (DeliveryStageSnapshot, error) {
	var response DeliveryStageSnapshot
	path := deliveryPath(stack, "/stages/"+url.PathEscape(stage)+"/"+url.PathEscape(action))
	err := pc.restCall(ctx, "POST", path, nil, request, &response)
	return response, err
}

func (pc *Client) GetDeliveryJobLogs(ctx context.Context, stack StackIdentifier, workflowRunID string) (DeliveryJobLogsResponse, error) {
	var response DeliveryJobLogsResponse
	err := pc.restCall(ctx, "GET", deliveryPath(stack, "/jobs/"+url.PathEscape(workflowRunID)+"/logs"), nil, nil, &response)
	return response, err
}

func (pc *Client) CompleteDeliveryTransition(ctx context.Context, stack StackIdentifier, transitionID string,
	request CompleteDeliveryTransitionRequest,
) error {
	return pc.restCall(ctx, "POST", deliveryPath(stack, "/transitions/"+url.PathEscape(transitionID)+"/complete"),
		nil, request, nil)
}

func (pc *Client) ApproveChangeRequest(ctx context.Context, org, requestID string, revision int, comment string) error {
	request := struct {
		RevisionNumber int    `json:"revisionNumber"`
		Comment        string `json:"comment,omitempty"`
	}{revision, comment}
	return pc.restCall(ctx, "POST", "/api/change-requests/"+url.PathEscape(org)+"/"+
		url.PathEscape(requestID)+"/approve", nil, request, nil)
}
