// Package optdestroy contains functional options to be used with stack destroy operations
// github.com/sdk/v2/go/x/auto Stack.Destroy(...optdestroy.Option)
package optdestroy

// Parallel is the number of resource operations to run in parallel at once during the destroy
// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
func Parallel(n int) Option {
	return optionFunc(func(opts *Options) {
		opts.Parallel = n
	})
}

// Message (optional) to associate with the destroy operation
func Message(message string) Option {
	return optionFunc(func(opts *Options) {
		opts.Message = message
	})
}

// Target specifies an exclusive list of resource URNs to destroy
func Target(urns []string) Option {
	return optionFunc(func(opts *Options) {
		opts.Target = urns
	})
}

// TargetDependents allows updating of dependent targets discovered but not specified in the Target list
func TargetDependents() Option {
	return optionFunc(func(opts *Options) {
		opts.TargetDependents = true
	})
}

// Option is a parameter to be applied to a Stack.Destroy() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// Parallel is the number of resource operations to run in parallel at once
	// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
	Parallel int
	// Message (optional) to associate with the destroy operation
	Message string
	// Specify an exclusive list of resource URNs to re
	Target []string
	// Allows updating of dependent targets discovered but not specified in the Target list
	TargetDependents bool
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
