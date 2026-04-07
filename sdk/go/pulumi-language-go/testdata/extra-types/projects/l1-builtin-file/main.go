package main

import (
	"crypto/sha256"
	"encoding/base64"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func filebase64OrPanic(path string) string {
	if fileData, err := os.ReadFile(path); err == nil {
		return base64.StdEncoding.EncodeToString(fileData[:])
	} else {
		panic(err.Error())
	}
}

func filebase64sha256OrPanic(path string) string {
	if fileData, err := os.ReadFile(path); err == nil {
		hashedData := sha256.Sum256([]byte(fileData))
		return base64.StdEncoding.EncodeToString(hashedData[:])
	} else {
		panic(err.Error())
	}
}

func readFileOrPanic(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	return string(data)
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		fileContent := readFileOrPanic("testfile.txt")
		fileB64 := filebase64OrPanic("testfile.txt")
		fileSha := filebase64sha256OrPanic("testfile.txt")
		ctx.Export("fileContent", pulumi.String(fileContent))
		ctx.Export("fileB64", pulumi.String(fileB64))
		ctx.Export("fileSha", pulumi.String(fileSha))
		return nil
	})
}
