package stack

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
)

func TestDisplayUpdatesJSONWithFailedDecryptedSecureObject(t *testing.T) {
	mockDecrypter := &FailingDecrypter{}

	update := []backend.UpdateInfo{
		{
			Version:   1,
			Kind:      apitype.UpdateKind(apitype.UpdateUpdate),
			StartTime: time.Now().Unix(),
			Message:   "Update with config that can't be decrypted",
			Environment: map[string]string{
				"PULUMI_ENV": "test",
			},
			Config: config.Map{
				config.MustMakeKey("test", "key2"): config.NewSecureObjectValue("object"),
			},
			Result:  backend.SucceededResult,
			EndTime: time.Now().Add(time.Minute).Unix(),
			ResourceChanges: display.ResourceChanges{
				display.StepOp(apitype.OpType(apitype.OpCreate)): 1,
			},
		},
	}

	err := displayUpdatesJSON(update, mockDecrypter)
	assert.NoError(t, err, "Expected config to not error and substitute ERROR_UNABLE_TO_DECRYPT")
}

type FailingDecrypter struct{}

func (m *FailingDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	return ciphertext, fmt.Errorf("bad value")
}

func (m *FailingDecrypter) BulkDecrypt(ctx context.Context, ciphertexts []string) (map[string]string, error) {
	return map[string]string{}, errors.New("fake failure")
}
