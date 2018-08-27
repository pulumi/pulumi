package errorutil

import "fmt"

var bailError = fmt.Errorf("bail")

func IsInternalError(err error) bool {
	return err != bailError
}

func Bail() error {
	return bailError
}
