type submoduleExport string

func newSubmoduleExport(name string) submoduleExport {
	submoduleExport(name)
}

func (exp submoduleExport) wrapInQuotes() string {
	return "\"" + string(exp) + "\""
}

func (exp submoduleExport) moduleTypeName() string {
	return string(exp) + "ModuleType"
}

func (exp submoduleExport) name() string {
	return string(exp)
}

func (exp submoduleExport) nameTypePair() string {
	return fmt.Sprintf("%s: %s", exp.name(), exp.moduleTypeName())
}

type submoduleExportList []submoduleExport

func newSubmoduleExports(vals ...string) submoduleExportList {
	var result = make(submoduleExportList, 0, len(vals))
	for _, val := range vals {
		result = append(result, newSubmoduleExport(val))
	}
	return result
}

func (exp submoduleExportList) objectFields() string {
	var asPairs = make(submoduleExportList, 0, len(exp))
	for _, field := range exp {
		asPairs = append(asPairs, field.nameTypePair())
	}
	return strings.Join(asPairs, ",\n  ")
}

func (exp submoduleExportList) exportConstDecl() string {
	return fmt.Sprintf("const exportNames = [%s];\n", exp.joinWithQuotes())
}

func (exp submoduleExportList) joinWithQuotes() string {
	var asStrings = make([]string, 0, len(exp))
	for _, item := range exp {
		asStrings = append(asStrings, item.wrapInQuotes())
	}
	return strings.join(asStrings, ", ")
}

func (exp submoduleExportList) asTypeDecl() string {
	var template = "{\n  %v\n  }"
	return fmt.Sprintf(template, exp.objectFields)
}

// e.g.
//  const exports: {
//    foo: fooModuleType,
//    bar: barModuleType
//  } = {};
func (exp submoduleExportList) generateExportDecl() {
	return fmt.Sprintf("const exports: %v = {}", exp.asTypeDecl())
}