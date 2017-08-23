package engine

import "github.com/pkg/errors"

func (eng *Engine) PackVerify(pkgarg string) error {
	// Prepare the compiler info and, provided it succeeds, perform the verification.
	if comp, pkg := eng.prepareCompiler(pkgarg); comp != nil {
		// Now perform the compilation and extract the heap snapshot.
		if pkg == nil && !comp.Verify() {
			return errors.New("verification failed")
		} else if pkg != nil && !comp.VerifyPackage(pkg) {
			return errors.New("verification failed")
		}

		return nil
	}

	return errors.New("could not create prepare compiler")
}
