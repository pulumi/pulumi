package python

import "fmt"

// Provides code for a method which will be placed in the program preamble if deemed
// necessary. Because many tasks in Go such as reading a file require extensive error
// handling, it is much prettier to encapsulate that error handling boilerplate as its
// own function in the preamble.
func getHelperMethodIfNeeded(functionName string, indent string) (string, bool) {
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
	default:
		return "", false
	}
}
