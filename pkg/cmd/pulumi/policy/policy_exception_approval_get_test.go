// Copyright 2026, Pulumi Corporation.
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

package policy

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestPolicyExceptionApprovalGet(t *testing.T) {
	t.Parallel()

	c := &mockPolicyExceptionClient{crCurrent: "pending", draft: sampleExceptionRequest(), gates: gateWith("bob")}
	stdout, _, err := runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID, "--org", "other-org")
	require.NoError(t, err)
	assert.Equal(t, "other-org", c.gotOrgFlag)
	assert.Equal(t, `ID:            `+approvalCRID+`
Policy pack:   aws-security
Status:        pending
Requested:     2026-11-18T09:00:00Z by alice
Expires:       7 days after it's created
Approved:      false
  Exceptions (not satisfied): 1 of 2 approvals (bob)
URL:           http://localhost:3000/other-org/web/prod?exception=`+approvalCRID+`

Policy Pack exceptions configuration:
{
  "reason": "Logs are public by design",
  "match": [
    {
      "policies": [
        "s3-no-public-read"
      ],
      "targets": [
        "urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs"
      ]
    }
  ]
}
`, stdout)

	// Once applied, it names the exception it created.
	c = &mockPolicyExceptionClient{
		crCurrent: "applied", draft: sampleExceptionRequest(),
		pages: []cloudapitype.ListPolicyExceptionsResponse{
			{
				Exceptions:        []cloudapitype.PolicyException{{ID: "EXC-3", ChangeRequestID: new("other")}},
				ContinuationToken: new("next"),
			},
			{Exceptions: []cloudapitype.PolicyException{{ID: "EXC-4", ChangeRequestID: new(approvalCRID)}}},
		},
	}
	stdout, _, err = runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID, "--output", "json")
	require.NoError(t, err)
	var got policyExceptionRequest
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.Equal(t, "EXC-4", got.ExceptionID)
	assert.Equal(t, "applied", got.Status)
	assert.Equal(t, "aws-security", got.Exception.PolicyPack)
	require.Len(t, c.gotListOpts, 2)
	assert.Equal(t, "aws-security", c.gotListOpts[0].PolicyPack)
	assert.Equal(t, []string{"active", "removed"}, c.gotListOpts[0].Status)
	assert.Equal(t, "next", c.gotListOpts[1].ContinuationToken)

	// Change requests for something else aren't shown.
	c = &mockPolicyExceptionClient{crEntityType: "environment"}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID)
	assert.EqualError(t, err, "change request "+approvalCRID+" isn't a policy exception request; "+
		"use `pulumi env approval` for environment change requests")
	c = &mockPolicyExceptionClient{noCR: true}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID)
	assert.EqualError(t, err, "policy exception request "+approvalCRID+" not found in acme")

	// An organization without the feature.
	c = &mockPolicyExceptionClient{err: &apitype.ErrorResponse{
		Code: http.StatusConflict, Message: `"acme" does not support policy-exceptions.`,
	}}
	_, _, err = runApprovalCmd(t, newPolicyExceptionApprovalGetCmd, c, approvalCRID)
	assert.EqualError(t, err, "policy exceptions aren't enabled for organization acme")
}
