package pulumi

import (
	"fmt"
	"io"
)

func Printf(format string, args ...interface{}) IntOutput {
	return All(args...).ApplyT(func(args []interface{}) (int, error) {
		return fmt.Printf(format, args...)
	}).(IntOutput)
}

func Fprintf(w io.Writer, format string, args ...interface{}) IntOutput {
	return All(args...).ApplyT(func(args []interface{}) (int, error) {
		return fmt.Fprintf(w, format, args...)
	}).(IntOutput)
}

func Sprintf(format string, args ...interface{}) StringOutput {
	return All(args...).ApplyT(func(args []interface{}) string {
		return fmt.Sprintf(format, args...)
	}).(StringOutput)
}
