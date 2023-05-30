package nodejs

import "fmt"

// Provides code for a method which will be placed in the program preamble if deemed
// necessary. Because many tasks in Go such as reading a file require extensive error
// handling, it is much prettier to encapsulate that error handling boilerplate as its
// own function in the preamble.
func getHelperMethodIfNeeded(functionName string, indent string) (string, bool) {
	switch functionName {
	case "filebase64sha256":
		return `function computeFilebase64sha256(path string) string {
	const fileData = Buffer.from(fs.readFileSync(path), 'binary')
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
	default:
		return "", false
	}
}
