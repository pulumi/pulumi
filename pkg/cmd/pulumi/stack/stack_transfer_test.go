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

package stack

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

type mockStackTransferClient struct {
	err      error
	called   bool
	gotStack client.StackIdentifier
	gotToOrg string
}

func (m *mockStackTransferClient) TransferStack(
	_ context.Context, stackID client.StackIdentifier, toOrg string,
) error {
	m.called = true
	m.gotStack = stackID
	m.gotToOrg = toOrg
	return m.err
}

func TestStackTransfer_Success(t *testing.T) {
	t.Parallel()

	c := &mockStackTransferClient{}
	var buf bytes.Buffer
	newID, err := runStackTransfer(t.Context(), &buf, c, testScheduleStackID, "other-org",
		true, false, display.Options{})
	require.NoError(t, err)

	assert.Equal(t, testScheduleStackID, c.gotStack)
	assert.Equal(t, "other-org", c.gotToOrg)
	assert.Equal(t, "other-org/my-project/dev", newID.String())
	assert.Contains(t, buf.String(), "Transferred my-org/my-project/dev to other-org/my-project/dev")
}

func TestStackTransfer_PromptConfirmed(t *testing.T) {
	t.Parallel()

	c := &mockStackTransferClient{}
	var buf bytes.Buffer
	_, err := runStackTransfer(t.Context(), &buf, c, testScheduleStackID, "other-org",
		false, true, display.Options{Color: colors.Never, Stdin: strings.NewReader("dev\n"), Stdout: &buf})
	require.NoError(t, err)
	assert.True(t, c.called)
}

func TestStackTransfer_PromptDeclined(t *testing.T) {
	t.Parallel()

	c := &mockStackTransferClient{}
	var buf bytes.Buffer
	_, err := runStackTransfer(t.Context(), &buf, c, testScheduleStackID, "other-org",
		false, true, display.Options{Color: colors.Never, Stdin: strings.NewReader("nope\n"), Stdout: &buf})
	require.Error(t, err)
	assert.False(t, c.called)
}

func TestStackTransfer_NonInteractiveRequiresYes(t *testing.T) {
	t.Parallel()

	c := &mockStackTransferClient{}
	_, err := runStackTransfer(t.Context(), &bytes.Buffer{}, c, testScheduleStackID, "other-org",
		false, false, display.Options{})
	require.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)
	assert.False(t, c.called)
}

func TestStackTransfer_SameOrg(t *testing.T) {
	t.Parallel()

	c := &mockStackTransferClient{}
	_, err := runStackTransfer(t.Context(), &bytes.Buffer{}, c, testScheduleStackID, "my-org",
		true, false, display.Options{})
	require.ErrorContains(t, err, "already in organization my-org")
	assert.False(t, c.called)
}

func TestStackTransfer_ClientError(t *testing.T) {
	t.Parallel()

	c := &mockStackTransferClient{err: errors.New("conflict")}
	_, err := runStackTransfer(t.Context(), &bytes.Buffer{}, c, testScheduleStackID, "other-org",
		true, false, display.Options{})
	require.ErrorContains(t, err, "transferring stack: conflict")
}
