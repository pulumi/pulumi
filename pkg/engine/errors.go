package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// DecryptError is the type of errors that arise when the engine can't decrypt a configuration key.
// The most common reason why this happens is that this key is being decrypted in a stack that's not the same
// one that encrypted it.
type DecryptError = engine.DecryptError

// Returns a tuple in which the second element is true if and only if any error
// in the given error's tree is a DecryptError. In that case, the first element
// will be the first DecryptError in the tree. In the event that there is no such
// DecryptError, the first element will be nil.
func AsDecryptError(err error) (*DecryptError, bool) {
	return engine.AsDecryptError(err)
}

