// Package optup contains functional options to be used with stack updates
// github.com/sdk/v2/go/x/auto Stack.Up(...optup.Option)
package optup

// Parallel is the number of resource operations to run in parallel at once during the update
// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
func Parallel(n int) Option {
	return optionFunc(func(opts *Options) {
		opts.Parallel = n
	})
}

// Message (optional) to associate with the update operation
func Message(message string) Option {
	return optionFunc(func(opts *Options) {
		opts.Message = message
	})
}

// ExpectNoChanges will cause the update to return an error if any changes occur
func ExpectNoChanges() Option {
	return optionFunc(func(opts *Options) {
		opts.ExpectNoChanges = true
	})
}

// Replace specifies an array of resource URNs to explicitly replace during the update
func Replace(urns []string) Option {
	return optionFunc(func(opts *Options) {
		opts.Replace = urns
	})
}

// Target specifies an exclusive list of resource URNs to update
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

// Option is a parameter to be applied to a Stack.Up() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// Parallel is the number of resource operations to run in parallel at once
	// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
	Parallel int
	// Message (optional) to associate with the update operation
	Message string
	// Return an error if any changes occur during this update
	ExpectNoChanges bool
	// Specify resources to replace
	Replace []string
	// Specify an exclusive list of resource URNs to update
	Target []string
	// Allows updating of dependent targets discovered but not specified in the Target list
	TargetDependents bool
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
