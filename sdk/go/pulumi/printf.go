package pulumi

import (
	"fmt"
	"io"
)

func Printf(format string, args ...AnyOutput) Output[int] {
	return Cast[int](
		All(args...).ApplyTErr(func(arr interface{}) (interface{}, error) {
			args := arr.([]interface{})
			return fmt.Printf(format, args...)
		}),
	)
}

func Fprintf(w io.Writer, format string, args ...AnyOutput) Output[int] {
	return Cast[int](
		All(args...).ApplyTErr(func(arr interface{}) (interface{}, error) {
			args := arr.([]interface{})
			return fmt.Fprintf(w, format, args...)
		}),
	)
}

func Sprintf(format string, args ...AnyOutput) Output[string] {
	return Cast[string](
		All(args...).ApplyT(func(arr interface{}) interface{} {
			args := arr.([]interface{})
			return fmt.Sprintf(format, args...)
		}),
	)
}
