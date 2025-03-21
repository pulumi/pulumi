// Copyright 2022-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package python

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// Provides code for a method which will be placed in the program preamble if deemed
// necessary. Because many tasks in Go such as reading a file require extensive error
// handling, it is much prettier to encapsulate that error handling boilerplate as its
// own function in the preamble.
func getHelperMethodIfNeeded(function *model.FunctionCallExpression, indent string) (string, bool) {
	functionName := function.Name
	switch functionName {
	case "filebase64sha256":
		return `def computeFilebase64sha256(path):
	fileData = open(path).read().encode()
	hashedData = hashlib.sha256(fileData.encode()).digest()
	return base64.b64encode(hashedData).decode()`, true
	case "notImplemented":
		return fmt.Sprintf(`
%sdef not_implemented(msg):
%s    raise NotImplementedError(msg)`, indent, indent), true
	case "singleOrNone":
		return fmt.Sprintf(
			`%sdef single_or_none(elements):
%s    if len(elements) != 1:
%s        raise Exception("single_or_none expected input list to have a single element")
%s    return elements[0]
`, indent, indent, indent, indent), true
	case "try":
		_, outputTry := function.Signature.ReturnType.(*model.OutputType)
		return generateTryFunction(outputTry, indent), true
	case "can":
		return fmt.Sprintf(`%[1]sdef can_(fn):
%[1]s    try:
%[1]s        _result = fn()
%[1]s        return True
%[1]s    except:
%[1]s        return False
`,
			indent,
		), true
	default:
		return "", false
	}
}

func generateTryFunction(outputTry bool, indent string) string {
	if outputTry {
		return generateOutputtyTry(indent)
	}

	return fmt.Sprintf(`%[1]sdef try_(*fns) -> typing.Any:
	%[1]s	for fn in fns:
	%[1]s		try:
	%[1]s			return fn()
	%[1]s		except:
	%[1]s			continue
	%[1]s	raise Exception("try: all parameters failed")
	`, indent)
}

// TODO NOW change back (All of them, this wont work until we do catch)
func generateOutputtyTry(indent string) string {
	return fmt.Sprintf(`%[1]sdef tryOutput_(*fns) -> pulumi.Output[typing.Any]:
%[1]s	if len(fns) == 0:
%[1]s		raise Exception("tryOutput: all parameters failed")
%[1]s
%[1]s	fn, *rest = fns
%[1]s	result_output = None
%[1]s	try:
%[1]s		result = fn()
%[1]s		result_output = pulumi.Output.from_input(result)
%[1]s	except:
%[1]s		return tryOutput_(*rest)
%[1]s
%[1]s	return result_output
%[1]s`, indent)
}
