package client

import client "github.com/pulumi/pulumi/sdk/v3/pkg/backend/httpstate/client"

// TemplatePublishOperationID uniquely identifies a template publish operation.
type TemplatePublishOperationID = client.TemplatePublishOperationID

// StartTemplatePublishRequest is the request body for starting a template publish operation.
type StartTemplatePublishRequest = client.StartTemplatePublishRequest

// StartTemplatePublishResponse is the response from initiating a template publish.
// It returns a presigned URL to upload the template archive.
type StartTemplatePublishResponse = client.StartTemplatePublishResponse

// TemplateUploadURLs contains the presigned URLs for uploading template artifacts.
type TemplateUploadURLs = client.TemplateUploadURLs

// PublishTemplateVersionCompleteRequest is the request body for completing a template publish operation.
type PublishTemplateVersionCompleteRequest = client.PublishTemplateVersionCompleteRequest

// PublishTemplateVersionCompleteResponse is the response from completing a template publish operation.
type PublishTemplateVersionCompleteResponse = client.PublishTemplateVersionCompleteResponse

// Client provides a slim wrapper around the Pulumi HTTP/REST API.
type Client = client.Client

// ListStacksFilter describes optional filters when listing stacks.
type ListStacksFilter = client.ListStacksFilter

// A [tar.Reader] that owns it's underlying data, and is thus responsible for closing it.
type TarReaderCloser = client.TarReaderCloser

type CreateUpdateDetails = client.CreateUpdateDetails

const CopilotRequestTimeout = client.CopilotRequestTimeout

// ErrNoPreviousDeployment is returned when there isn't a previous deployment.
var ErrNoPreviousDeployment = client.ErrNoPreviousDeployment

// NewClient creates a new Pulumi API client with the given URL and API token.
func NewClient(apiURL, apiToken string, insecure bool, d diag.Sink) *Client {
	return client.NewClient(apiURL, apiToken, insecure, d)
}

