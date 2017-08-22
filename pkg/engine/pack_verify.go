package engine

import "github.com/pkg/errors"

func PackVerify(pkgarg string) error {
	// Prepare the compiler info and, provided it succeeds, perform the verification.
	if comp, pkg := prepareCompiler(pkgarg); comp != nil {
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
