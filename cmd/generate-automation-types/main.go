package main

import (
	"fmt"
	parser "github.com/yoheimuta/go-protoparser/v4"
	"os"
)

func main() {
	args := os.Args[1:]
	if len(args) < 3 {
		fmt.Println("Usage: generate-automation-types <protofile> <language> <out>")
		return
	}

	protoFile := args[0]
	language := args[1]
	out := args[2]

	file, err := os.OpenFile(protoFile, os.O_RDONLY, 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer file.Close()

	protoDef, err := parser.Parse(file)

	if err != nil {
		fmt.Println(err)
		return
	}

	moduleInfo := readModuleInfo(protoDef)

	if language == "python" || language == "py" {
		fmt.Println("Generating python code")
		code := emitPythonTypes(moduleInfo)
		err = os.WriteFile(out, []byte(code), 0755)
	} else if language == "typescript" || language == "ts" {
		fmt.Println("Generating typescript code")
		code := emitTypescriptTypes(moduleInfo)
		err = os.WriteFile(out, []byte(code), 0755)
	} else {
		fmt.Printf("Unsupported language '%s'\n", language)
		return
	}

	if err != nil {
		fmt.Println(err)
		return
	}
}
