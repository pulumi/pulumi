// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMutationState(t *testing.T) {
	t.Parallel()
	liveStatuses := []MutationStatus{
		ResourceStatusCreated,
		ResourceStatusUpdated,
	}

	notLiveStatuses := []MutationStatus{
		ResourceStatusUnspecified,
		ResourceStatusCreating,
		ResourceStatusUpdating,
		ResourceStatusPendingDeletion,
		ResourceStatusDeleting,
	}

	for _, live := range liveStatuses {
		t.Run(string(live), func(tt *testing.T) {
			assert.True(tt, live.Live())
		})
	}

	for _, notLive := range notLiveStatuses {
		t.Run(string(notLive), func(tt *testing.T) {
			assert.False(tt, notLive.Live())
		})
	}
}
