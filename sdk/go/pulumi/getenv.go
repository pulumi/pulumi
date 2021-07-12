package pulumi

import "os"

func Getenv(key string) String {
	return String(os.Getenv(key))
}
