package pulumi

import "os"

func Getenv(key string) StringOutput {
	return String(
		os.Getenv(key),
	).ToStringOutput()
}
