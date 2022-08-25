package nodejs

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func ExampleModulesExport() {
	var buffer bytes.Buffer
	var submodExp = newSubmoduleExportList("kubernetes", "docker")
	submodExp.WriteSrc(&buffer, "exports")
	// To make this exmaple consistant with tabs vs spaces, we
	// strip out leading whitespace from each line.
	for _, line := range strings.Split(buffer.String(), "\n") {
		fmt.Println(strings.TrimSpace(line))
	}
	// Output:
	// import * as kubernetesModule from "./kubernetes";
	// export const kubernetes: typeof kubernetesModule = {} as typeof kubernetesModule;
	// import * as dockerModule from "./docker";
	// export const docker: typeof dockerModule = {} as typeof dockerModule;
	//
	// utilities.lazy_load_all(
	// exports,
	// ["kubernetes", "docker"],
	// );
}

// TestJoinMods is a smoke test to prove we're
// able to build a JS array from a list of modules.
func TestJoinMods(t *testing.T) {
	t.Parallel()
	var submodExp = newSubmoduleExportList("apples", "oranges")
	var jsArray = submodExp.joinMods()
	assert.Equal(t, jsArray, `["apples", "oranges"]`)
}

func TestSubmodExport(t *testing.T) {
	t.Parallel()

	var testCases = []struct {
		moduleName, // the input
		// The rest of these are function outputs
		fileName,
		typeName,
		quoted,
		qualifiedTypeName,
		importStmt,
		exportStmt string
	}{{
		moduleName:        `foobar`,
		fileName:          `"./foobar"`,
		typeName:          `foobarModule`,
		quoted:            `"foobar"`,
		qualifiedTypeName: `typeof foobarModule`,
		importStmt:        `import * as foobarModule from "./foobar";`,
		exportStmt:        `export const foobar: typeof foobarModule = {} as typeof foobarModule;`,
	}, {
		moduleName:        `myModule`,
		fileName:          `"./myModule"`,
		typeName:          `myModuleModule`,
		quoted:            `"myModule"`,
		qualifiedTypeName: `typeof myModuleModule`,
		importStmt:        `import * as myModuleModule from "./myModule";`,
		exportStmt:        `export const myModule: typeof myModuleModule = {} as typeof myModuleModule;`,
	}, {
		moduleName:        `LEGAL_NODEJS_ID`,
		fileName:          `"./LEGAL_NODEJS_ID"`,
		typeName:          `LEGAL_NODEJS_IDModule`,
		quoted:            `"LEGAL_NODEJS_ID"`,
		qualifiedTypeName: `typeof LEGAL_NODEJS_IDModule`,
		importStmt:        `import * as LEGAL_NODEJS_IDModule from "./LEGAL_NODEJS_ID";`,
		exportStmt:        `export const LEGAL_NODEJS_ID: typeof LEGAL_NODEJS_IDModule = {} as typeof LEGAL_NODEJS_IDModule;`,
	}}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("submodExport-%s", tc.moduleName), func(t *testing.T) {
			var submod = newSubmoduleExport(tc.moduleName)
			var expected, observed string
			// Compare file names
			expected, observed = tc.fileName, submod.fileName()
			assert.Equal(t, expected, observed)
			// Compare type names
			expected, observed = tc.typeName, submod.typeName()
			assert.Equal(t, expected, observed)
			// Compare quoted names
			expected, observed = tc.quoted, submod.quoted()
			assert.Equal(t, expected, observed)
			// Compare qualified type names
			expected, observed = tc.qualifiedTypeName, submod.qualifiedTypeName()
			assert.Equal(t, expected, observed)
			// Compare import statements
			expected, observed = tc.importStmt, submod.genImport()
			assert.Equal(t, expected, observed)
			// Compare export statements.
			expected, observed = tc.exportStmt, submod.genExportLocal()
			assert.Equal(t, expected, observed)
		})
	}
}
