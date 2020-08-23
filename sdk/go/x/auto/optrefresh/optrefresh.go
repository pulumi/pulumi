// Package optrefresh contains functional options to be used with stack refresh operations
// github.com/sdk/v2/go/x/auto Stack.Refresh(...optrefresh.Option)
package optrefresh

// Parallel is the number of resource operations to run in parallel at once during the refresh
// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
func Parallel(n int) Option {
	return optionFunc(func(opts *Options) {
		opts.Parallel = n
	})
}

// Message (optional) to associate with the refresh operation
func Message(message string) Option {
	return optionFunc(func(opts *Options) {
		opts.Message = message
	})
}

// Target specifies an exclusive list of resource URNs to refresh
func Target(urns []string) Option {
	return optionFunc(func(opts *Options) {
		opts.Target = urns
	})
}

// Option is a parameter to be applied to a Stack.Refresh() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// Parallel is the number of resource operations to run in parallel at once
	// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
	Parallel int
	// Message (optional) to associate with the refresh operation
	Message string
	// Specify an exclusive list of resource URNs to re
	Target []string
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
