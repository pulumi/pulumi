package auto

// TODO tag with yaml:... https://github.com/go-yaml/yaml

// Project is a description of a pulumi project and corresponding source code
type Project struct {
	// Name of the project
	Name string
	//
	SourcePath string
	// Overrides is an optional set of values to overwrite in pulumi.yaml
	Overrides *ProjectOverrides
}

// Stack is a description of a pulumi stack
type Stack struct {
	// Name of the the stack
	Name string
	// Project is a description of the project to execute
	Project Project
	// Overrides is an optional set of values to overwrite in pulumi.<stack>.yaml
	Overrides *StackOverrides
}

// TODO - expand to match https://www.pulumi.com/docs/intro/concepts/project/
//      - backend, complete runtime, config dir, etc
// Project is an optional set of values to be merged with
// the existing pulumi.yaml
type ProjectOverrides struct {
	// Runtime is the language runtime for the project
	Runtime *Runtime
	// Main is the entrypoint of the program
	Main *string
}

// StackOverrides is an optional set of values to be merged with
// the existing pulumi.<stackName>.yaml
type StackOverrides struct {
	// Config is an optional config bag to `pulumi config set`
	Config map[string]string
	// Secrets is an optional config bag to `pulumi config set --secret`
	Secrets map[string]string
	// SecretsProvider is this stack's secrets provider.
	SecretsProvider *string
	// EncryptedKey is the KMS-encrypted ciphertext for the data key used for secrets encryption.
	// Only used for cloud-based secrets providers.
	EncryptedKey *string
	// EncryptionSalt is this stack's base64 encoded encryption salt.  Only used for
	// passphrase-based secrets providers.
	EncryptionSalt *string
}

type Runtime int

const (
	Node Runtime = iota
	Python
	Go
	Dotnet
)

func (r Runtime) String() string {
	return []string{"nodejs", "python", "go", "dotnet"}[r]
}

/*
1. pulumi.yaml is shared mutable state
2. pulumi.stack.yaml could possibly be mutated concurrently, but only one concurrent `pulumi up` can happen per stack
*/
