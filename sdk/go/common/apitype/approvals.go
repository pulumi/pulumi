package apitype

import "fmt"

// CreateChangeRequestRequest is used to create a new change request
type CreateChangeRequestRequest struct {
	// The type of entity the change request targets
	EntityType string `json:"entityType" yaml:"entityType"`
	// The ID of the entity being changed. Mutually exclusive with qualifiedName.
	EntityID *string `json:"entityID,omitempty" yaml:"entityID,omitempty"`
	// The qualified name of the entity (e.g., 'org/project/env'). Mutually exclusive with entityID.
	QualifiedName *string `json:"qualifiedName,omitempty" yaml:"qualifiedName,omitempty"`
	// The type of action to perform
	ActionType string `json:"actionType" yaml:"actionType"`
	// Optional constraint key for uniqueness
	ConstraintKey *string `json:"constraintKey,omitempty" yaml:"constraintKey,omitempty"`
	// The payload for the change request (JSON object)
	Payload any `json:"payload" yaml:"payload"`
	// Description/justification for the change request
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	// Initial state of the change request: 'draft' or 'pending'. Defaults to 'draft' if not specified.
	InitialState *string `json:"initialState,omitempty" yaml:"initialState,omitempty"`
}

// CreateChangeRequestResponse contains the ID and ETag of the newly created change request
type CreateChangeRequestResponse struct {
	// The ID of the created change request
	ChangeRequestID string `json:"changeRequestID" yaml:"changeRequestID"`
	// The ETag of the first revision
	Etag string `json:"etag" yaml:"etag"`
}

type ChangeGateTargetEntityType string

const (
	ChangeGateTargetEntityTypeEnvironment ChangeGateTargetEntityType = "environment"
	ChangeGateTargetEntityTypeStack       ChangeGateTargetEntityType = "stack"
)

func (v ChangeGateTargetEntityType) IsValid() bool {
	switch v {
	case ChangeGateTargetEntityTypeEnvironment:
		return true
	case ChangeGateTargetEntityTypeStack:
		return true
	}

	return false
}

func (v ChangeGateTargetEntityType) openapiName() string {
	switch v {
	case ChangeGateTargetEntityTypeEnvironment:
		return "Environment"
	case ChangeGateTargetEntityTypeStack:
		return "Stack"
	}

	return ""
}

func (v ChangeGateTargetEntityType) GoString() string {
	if s := v.openapiName(); s != "" {
		return "apitype.ChangeGateTargetEntityType" + s
	}
	return fmt.Sprintf("apitype.ChangeGateTargetEntityType(%#v)", string(v))
}
