package apitype

import "github.com/pulumi/pulumi/pkg/tokens"

// UpdateProgramRequest is the request type for updating (aka deploying) a Pulumi program.
type UpdateProgramRequest struct {
	// Properties from the Project file. Subset of pack.Package.
	Name        tokens.PackageName `json:"name"`
	Runtime     string             `json:"runtime"`
	Main        string             `json:"main"`
	Description string             `json:"description"`

	// Configuration values.
	Config map[tokens.ModuleMember]string `json:"config"`
}

// UpdateProgramResponse is the result of an update program request.
type UpdateProgramResponse struct {
	// UpdateID is the opaque identifier of the requested update. This value is needed to begin
	// an update, as well as poll for its progress.
	UpdateID string `json:"updateID"`

	// UploadURL is a URL the client can use to upload their program's contents into. Ignored
	// for destroys.
	UploadURL string `json:"uploadURL"`
}

// StartUpdateResponse is the result of the command to start an update.
type StartUpdateResponse struct {
	// Version is the version of the program once the update is complete.
	// (Will be the current, unchanged value for previews.)
	Version int `json:"version"`
}
