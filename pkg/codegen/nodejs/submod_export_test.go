package nodejs

import (
	"fmt"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSubmodExport(t *testing.T) {
	t.Parallel()
	
	var testCases =[] struct{
		moduleName, 
		fileName,
		typeName,
		qualifiedTypeName,
		importStmt,
		exportStmt string
	}{{
		moduleName: `foobar`,
		fileName: `"./foobar"`,
		typeName: `foobarModule`,
		qualifiedTypeName: `typeof foobarModule`,
		importStmt: `import * as foobarModule from "./foobar";`,
		exportStmt: `export const foobar: typeof foobarModule = {} as typeof foobarModule;`,
	}, {	
		moduleName: `myModule`,
		fileName: `"./myModule"`,
		typeName: `myModuleModule`,
		qualifiedTypeName: `typeof myModuleModule`,
		importStmt: `import * as myModuleModule from "./myModule";`,
		exportStmt: `export const myModule: typeof myModuleModule = {} as typeof myModuleModule;`,
	}, {	
		moduleName: `LEGAL_NODEJS_ID`,
		fileName: `"./LEGAL_NODEJS_ID"`,
		typeName: `LEGAL_NODEJS_IDModule`,
		qualifiedTypeName: `typeof LEGAL_NODEJS_IDModule`,
		importStmt: `import * as LEGAL_NODEJS_IDModule from "./LEGAL_NODEJS_ID";`,
		exportStmt: `export const LEGAL_NODEJS_ID: typeof LEGAL_NODEJS_IDModule = {} as typeof LEGAL_NODEJS_IDModule;`,
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
