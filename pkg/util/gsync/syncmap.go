package gsync

import gsync "github.com/pulumi/pulumi/sdk/v3/pkg/util/gsync"

// Map is like a Go map[K]V but is safe for concurrent use by multiple goroutines without additional
// locking or coordination. Loads, stores, and deletes run in amortized constant time.
type Map = gsync.Map

