package main

import (
	"fmt"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
)

func main() {
	data := map[string]interface{}{
		"name": "test-stack",
		"resources": []map[string]interface{}{
			{
				"type": "aws:s3/bucket:Bucket",
				"name": "my-bucket",
				"properties": map[string]interface{}{
					"bucket": "my-test-bucket",
					"acl":    "private",
				},
			},
		},
	}

	// Test pretty JSON
	prettyBytes, err := encoding.JSON.Marshal(data)
	if err != nil {
		panic(err)
	}
	fmt.Println("Pretty JSON:")
	fmt.Println(string(prettyBytes))
	fmt.Printf("Size: %d bytes\n\n", len(prettyBytes))

	// Test compact JSON
	compactBytes, err := encoding.CompactJSON.Marshal(data)
	if err != nil {
		panic(err)
	}
	fmt.Println("Compact JSON:")
	fmt.Println(string(compactBytes))
	fmt.Printf("Size: %d bytes\n\n", len(compactBytes))

	fmt.Printf("Space saved: %d bytes (%.1f%%)\n", 
		len(prettyBytes)-len(compactBytes),
		float64(len(prettyBytes)-len(compactBytes))/float64(len(prettyBytes))*100)
}