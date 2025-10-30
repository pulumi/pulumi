package plan

import plan "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/plan"

func Write(path string, plan_ *deploy.Plan, enc config.Encrypter, showSecrets bool) error {
	return plan.Write(path, plan_, enc, showSecrets)
}

func Read(path string, dec config.Decrypter) (*deploy.Plan, error) {
	return plan.Read(path, dec)
}

