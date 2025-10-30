package channel

import channel "github.com/pulumi/pulumi/sdk/v3/pkg/channel"

// FilterRead reads every item from the input channel and writes it to the output channel if the given filter function
// returns true for it.
func FilterRead(ch <-chan T, f func(T) bool) <-chan T {
	return channel.FilterRead(ch, f)
}

