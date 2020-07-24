package auto

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
)

// Lifted from
// https://github.com/pulumi/pulumi/blob/45d2fa95d60be71d170d15c6d9a24274b80ddc91/pkg/backend/display/json.go#L229

type PreviewStep struct {
	// Op is the kind of operation being performed.
	Op string `json:"op"`
	// URN is the resource being affected by this operation.
	URN resource.URN `json:"urn"`
	// Provider is the provider that will perform this step.
	Provider string `json:"provider,omitempty"`
	// OldState is the old state for this resource, if appropriate given the operation type.
	OldState *apitype.ResourceV3 `json:"oldState,omitempty"`
	// NewState is the new state for this resource, if appropriate given the operation type.
	NewState *apitype.ResourceV3 `json:"newState,omitempty"`
	// DiffReasons is a list of keys that are causing a diff (for updating steps only).
	DiffReasons []resource.PropertyKey `json:"diffReasons,omitempty"`
	// ReplaceReasons is a list of keys that are causing replacement (for replacement steps only).
	ReplaceReasons []resource.PropertyKey `json:"replaceReasons,omitempty"`
	// DetailedDiff is a structured diff that indicates precise per-property differences.
	DetailedDiff map[string]PropertyDiff `json:"detailedDiff"`
}

// propertyDiff contains information about the difference in a single property value.
type PropertyDiff struct {
	// Kind is the kind of difference.
	Kind string `json:"kind"`
	// InputDiff is true if this is a difference between old and new inputs instead of old state and new inputs.
	InputDiff bool `json:"inputDiff"`
}

type PreviewResult struct {
	Steps         []PreviewStep  `json:"steps"`
	ChangeSummary map[string]int `json:"changeSummary"`
}

func (s *stack) Preview() (PreviewResult, error) {
	var pResult PreviewResult

	err := s.initOrSelectStack()
	if err != nil {
		return pResult, err
	}
	return s.preview()
}

func (s *stack) preview() (PreviewResult, error) {
	var pResult PreviewResult

	var stdout, stderr string
	var code int
	var err error
	if s.InlineSource != nil {
		stdout, stderr, err = s.host(true /*isPreview*/)
		if err != nil {
			return pResult, newAutoError(err, stdout, stderr, code)
		}
	} else {
		stdout, stderr, code, err = s.runCmd("pulumi", "preview", "--json")
		if err != nil {
			return pResult, newAutoError(errors.Wrap(err, "failed to run preview"), stdout, stderr, code)
		}
	}

	err = json.Unmarshal([]byte(stdout), &pResult)
	if err != nil {
		return pResult, errors.Wrap(err, "unable to unmarshal preview result")
	}

	return pResult, nil
}
