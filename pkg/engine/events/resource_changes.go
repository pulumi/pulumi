package events

// ResourceChanges contains the aggregate resource changes by operation type.
type ResourceChanges map[StepOp]int

// HasChanges returns true if there are any non-same changes in the resulting summary.
func (changes ResourceChanges) HasChanges() bool {
	var c int
	for op, count := range changes {
		if op != OpSame &&
			op != OpRead &&
			op != OpReadDiscard &&
			op != OpReadReplacement {
			c += count
		}
	}
	return c > 0
}
