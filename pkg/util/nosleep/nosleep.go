package nosleep

import nosleep "github.com/pulumi/pulumi/sdk/v3/pkg/util/nosleep"

type DoneFunc = nosleep.DoneFunc

// KeepRunning attempts to prevent idle sleep on the system.  This is useful for long running processes, e.g. updates
// that should not be interrupted by the system going to sleep.  It's not guaranteed to work on all systems or at all
// times.  Users can still manually put the system to sleep.
func KeepRunning() DoneFunc {
	return nosleep.KeepRunning()
}

