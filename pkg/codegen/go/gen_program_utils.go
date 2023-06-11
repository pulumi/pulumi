package gen

import (
	"fmt"
	"io"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type promptToInputArrayHelper struct {
	destType string
}

var primitives = map[string]string{
	"String":  "string",
	"Bool":    "bool",
	"Int":     "int",
	"Int64":   "int64",
	"Float64": "float64",
}

func (p *promptToInputArrayHelper) generateHelperMethod(w io.Writer) {
	promptType := p.getPromptItemType()
	inputType := p.getInputItemType()
	fnName := p.getFnName()
	fmt.Fprintf(w, "func %s(arr []%s) %s {\n", fnName, promptType, p.destType)
	fmt.Fprintf(w, "var pulumiArr %s\n", p.destType)
	fmt.Fprintf(w, "for _, v := range arr {\n")
	fmt.Fprintf(w, "pulumiArr = append(pulumiArr, %s(v))\n", inputType)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintf(w, "return pulumiArr\n")
	fmt.Fprintf(w, "}\n")
}

func (p *promptToInputArrayHelper) getFnName() string {
	parts := strings.Split(p.destType, ".")
	contract.Assertf(len(parts) == 2, "promptToInputArrayHelper destType expected to have two parts.")
	return fmt.Sprintf("to%s%s", Title(parts[0]), Title(parts[1]))
}

func (p *promptToInputArrayHelper) getPromptItemType() string {
	inputType := p.getInputItemType()
	parts := strings.Split(inputType, ".")
	contract.Assertf(len(parts) == 2, "promptToInputArrayHelper destType expected to have two parts.")
	typ := parts[1]
	if t, ok := primitives[typ]; ok {
		return t
	}

	return typ
}

func (p *promptToInputArrayHelper) getInputItemType() string {
	return strings.TrimSuffix(p.destType, "Array")
}

// Provides code for a method which will be placed in the program preamble if deemed
// necessary. Because many tasks in Go such as reading a file require extensive error
// handling, it is much prettier to encapsulate that error handling boilerplate as its
// own function in the preamble.
func getHelperMethodIfNeeded(functionName string, indent string) (string, bool) {
	switch functionName {
	case "readFile":
		return `func readFileOrPanic(path string) pulumi.StringPtrInput {
				data, err := os.ReadFile(path)
				if err != nil {
					panic(err.Error())
				}
				return pulumi.String(string(data))
			}`, true
	case "filebase64":
		return `func filebase64OrPanic(path string) pulumi.StringPtrInput {
					if fileData, err := os.ReadFile(path); err == nil {
						return pulumi.String(base64.StdEncoding.EncodeToString(fileData[:]))
					} else {
						panic(err.Error())
					}
				}`, true
	case "filebase64sha256":
		return `func filebase64sha256OrPanic(path string) pulumi.StringPtrInput {
					if fileData, err := os.ReadFile(path); err == nil {
						hashedData := sha256.Sum256([]byte(fileData))
						return pulumi.String(base64.StdEncoding.EncodeToString(hashedData[:]))
					} else {
						panic(err.Error())
					}
				}`, true
	case "sha1":
		return `func sha1Hash(input string) string {
				hash := sha1.Sum([]byte(input))
				return hex.EncodeToString(hash[:])
			}`, true
	case "notImplemented":
		return fmt.Sprintf(`
%sfunc notImplemented(message string) pulumi.AnyOutput {
%s  panic(message)
%s}`, indent, indent, indent), true
	case "singleOrNone":
		return fmt.Sprintf(`%sfunc singleOrNone[T any](elements []T) T {
%s	if len(elements) != 1 {
%s		panic(fmt.Errorf("singleOrNone expected input slice to have a single element"))
%s	}
%s	return elements[0]
%s}`, indent, indent, indent, indent, indent, indent), true
	default:
		return "", false
	}
}
