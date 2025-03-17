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

package nodejs

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// Provides code for a method which will be placed in the program preamble if deemed
// necessary. Because many tasks in Go such as reading a file require extensive error
// handling, it is much prettier to encapsulate that error handling boilerplate as its
// own function in the preamble.
func getHelperMethodIfNeeded(function *model.FunctionCallExpression, indent string) (string, bool) {
	switch function.Name {
	case "filebase64sha256":
		return `function computeFilebase64sha256(path: string): string {
	const fileData = Buffer.from(fs.readFileSync(path, 'binary'))
	return crypto.createHash('sha256').update(fileData).digest('hex')
}`, true
	case "notImplemented":
		return fmt.Sprintf(
			`%sfunction notImplemented(message: string) {
%s    throw new Error(message);
%s}`, indent, indent, indent), true
	case "singleOrNone":
		return fmt.Sprintf(
			`%sfunction singleOrNone<T>(elements: pulumi.Input<T>[]): pulumi.Input<T> {
%s    if (elements.length != 1) {
%s        throw new Error("singleOrNone expected input list to have a single element");
%s    }
%s    return elements[0];
%s}`, indent, indent, indent, indent, indent, indent), true
	case "mimeType":
		return fmt.Sprintf(`%sfunction mimeType(path: string): string {
%s    throw new Error("mimeType not implemented, use the mime or mime-types package instead");
%s}`, indent, indent, indent), true
	case "try":
		_, outputTry := function.Signature.ReturnType.(*model.OutputType)
		return generateTryFunction(outputTry, indent), true
	case "can":
		// Much like try, but instead of returning the result only returns true or
		// false if the one argument has no error.  The "too safe" problem
		// described above exists for can as well.
		return fmt.Sprintf(`%[1]sfunction can_(
%[1]s    fn: () => unknown
%[1]s): boolean {
%[1]s    try {
%[1]s        const result = fn();
%[1]s        if (result === undefined) {
%[1]s            return false;
%[1]s        }
%[1]s        return true;
%[1]s    } catch (e) {
%[1]s        return false;
%[1]s    }
%[1]s}
`,
			indent,
		), true
	default:
		return "", false
	}
}

// During code generation, it's possible that we'll generate expression code that is "too safe" as arguments to try.
// E.g. given some PCL code `try(a.b.c, "fallback")`, where perhaps `a.b` is not defined, we'd ideally generate
// `a.b.c` in TypeScript, which will throw and hit the `catch` block. However, depending on the inferred optionality
// of the expressions involved, we may generate e.g. `a?.b?.c`, which instead of throwing will return `undefined`.
// We thus check for this explicitly in our helper. This should be safe, since `undefined` is _not_ strictly equal
// to `null`, and `null` is the "official no-value" value for PCL.
func generateTryFunction(outputTry bool, indent string) string {
	if outputTry {
		return generateOutputtyTryFunction(indent)
	}

	return fmt.Sprintf(`%[1]sfunction try_(
%[1]s    ...fns: Array<() => unknown>
%[1]s): any {
%[1]s    for (const fn of fns) {
%[1]s        try {
%[1]s            const result = fn();
%[1]s            if (result === undefined) {
%[1]s                continue;
%[1]s            }
%[1]s            return result;
%[1]s        } catch (e) {
%[1]s            continue;
%[1]s        }
%[1]s    }
%[1]s    throw new Error("try: all parameters failed");
%[1]s}
`, indent,
	)
}

func generateOutputtyTryFunction(indent string) string {
	return fmt.Sprintf(`%[1]sfunction tryOutput_(
%[1]s    ...fns: Array<() => pulumi.Input<unknown>>
%[1]s): pulumi.Output<any> {
%[1]s    if (fns.length === 0) {
%[1]s	       throw new Error("try: all parameters failed");
%[1]s    }
%[1]s
%[1]s    const [fn, ...rest] = fns;
%[1]s    let resultOutput: pulumi.Output<any> | undefined;
%[1]s    try {
%[1]s        const result = fn();
%[1]s        if (result === undefined) {
%[1]s            return tryOutput_(...rest);
%[1]s        }
%[1]s        resultOutput = pulumi.output(result);
%[1]s    } catch {
%[1]s	       return tryOutput_(...rest);
%[1]s    }
%[1]s
%[1]s    // @ts-ignore
%[1]s	   return resultOutput;
%[1]s}
`, indent)
}
