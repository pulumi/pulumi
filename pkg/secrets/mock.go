package secrets

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

type MockSecretsManager struct {
	TypeF      func() string
	StateF     func() json.RawMessage
	EncrypterF func() (config.Encrypter, error)
	DecrypterF func() (config.Decrypter, error)
}

var _ Manager = &MockSecretsManager{}

func (msm *MockSecretsManager) Type() string {
	if msm.TypeF != nil {
		return msm.TypeF()
	}

	panic("not implemented")
}

func (msm *MockSecretsManager) State() json.RawMessage {
	if msm.StateF != nil {
		return msm.StateF()
	}

	panic("not implemented")
}

func (msm *MockSecretsManager) Encrypter() (config.Encrypter, error) {
	if msm.EncrypterF != nil {
		return msm.EncrypterF()
	}

	panic("not implemented")
}

func (msm *MockSecretsManager) Decrypter() (config.Decrypter, error) {
	if msm.DecrypterF != nil {
		return msm.DecrypterF()
	}

	panic("not implemented")
}

type MockEncrypter struct {
	EncryptValueF func() string
}

func (me *MockEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	if me.EncryptValueF != nil {
		return me.EncryptValueF(), nil
	}

	return "", errors.New("mock value not provided")
}

type MockDecrypter struct {
	DecryptValueF func() string
	BulkDecryptF  func() map[string]string
}

func (md *MockDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	if md.DecryptValueF != nil {
		return md.DecryptValueF(), nil
	}

	return "", errors.New("mock value not provided")
}

func (md *MockDecrypter) BulkDecrypt(ctx context.Context, ciphertexts []string) (map[string]string, error) {
	if md.BulkDecryptF != nil {
		return md.BulkDecryptF(), nil
	}

	return nil, errors.New("mock value not provided")
}
