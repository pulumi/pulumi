package pulumi

import (
	"fmt"
	"log"
	"os"
)

func dbgPrintf(format string, args ...interface{}) {
	f, err := os.OpenFile("/tmp/pulumi.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintf(f, format, args...)
}
