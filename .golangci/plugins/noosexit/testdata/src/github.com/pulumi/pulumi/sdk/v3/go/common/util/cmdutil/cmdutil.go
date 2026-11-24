package cmdutil

// Stub of the real cmdutil exit helpers, used only so the analyzer test can
// resolve calls to them by import path.

func Exit(err error) {}

func ExitError(msg string) {}
