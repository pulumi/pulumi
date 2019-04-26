package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"

	"github.com/pulumi/pulumi/pkg/diag"

	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/secrets"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

const Type = "pulumi"

// cloudCrypter is an encrypter/decrypter that uses the Pulumi cloud to encrypt/decrypt a stack's secrets.
type cloudCrypter struct {
	client *client.Client
	stack  client.StackIdentifier
}

func newCloudCrypter(client *client.Client, stack client.StackIdentifier) config.Crypter {
	return &cloudCrypter{client: client, stack: stack}
}

func (c *cloudCrypter) EncryptValue(plaintext string) (string, error) {
	ciphertext, err := c.client.EncryptValue(context.Background(), c.stack, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *cloudCrypter) DecryptValue(cipherstring string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherstring)
	if err != nil {
		return "", err
	}
	plaintext, err := c.client.DecryptValue(context.Background(), c.stack, ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

type cloudSecretsManagerState struct {
	URL     string `json:"url,omitempty"`
	Owner   string `json:"owner"`
	Project string `json:"project"`
	Stack   string `json:"stack"`
}

var _ secrets.Manager = &cloudSecretsManager{}

type cloudSecretsManager struct {
	state   cloudSecretsManagerState
	crypter config.Crypter
}

func (sm *cloudSecretsManager) Type() string {
	return Type
}

func (sm *cloudSecretsManager) State() interface{} {
	return sm.state
}

func (sm *cloudSecretsManager) Decrypter() (config.Decrypter, error) {
	contract.Assert(sm.crypter != nil)
	return sm.crypter, nil
}

func (sm *cloudSecretsManager) Encrypter() (config.Encrypter, error) {
	contract.Assert(sm.crypter != nil)
	return sm.crypter, nil
}

func NewCloudSecretsManager(c *client.Client, id client.StackIdentifier) (secrets.Manager, error) {
	return &cloudSecretsManager{
		state: cloudSecretsManagerState{
			URL:     c.URL(),
			Owner:   id.Owner,
			Project: id.Project,
			Stack:   id.Stack,
		},
		crypter: newCloudCrypter(c, id),
	}, nil
}

type provider struct{}

var _ secrets.ManagerProvider = &provider{}

func NewProvider() secrets.ManagerProvider {
	return &provider{}
}

func (p *provider) FromState(state json.RawMessage) (secrets.Manager, error) {
	var s cloudSecretsManagerState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil, errors.Wrap(err, "unmarshalling state")
	}

	token, err := workspace.GetAccessToken(s.URL)
	if err != nil {
		return nil, errors.Wrap(err, "getting access token")
	}

	if token == "" {
		return nil, errors.Errorf("could not find access token for %s, have you logged in?", s.URL)
	}

	id := client.StackIdentifier{
		Owner:   s.Owner,
		Project: s.Project,
		Stack:   s.Stack,
	}
	c := client.NewClient(s.URL, token, diag.DefaultSink(ioutil.Discard, ioutil.Discard, diag.FormatOptions{}))

	return &cloudSecretsManager{
		state:   s,
		crypter: newCloudCrypter(c, id),
	}, nil
}
