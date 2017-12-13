// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ints

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

// TestDiffs tests many combinations of creates, updates, deletes, replacements, and checks the
// output of the command against an expected baseline.
func TestDiffs(t *testing.T) {
	opts := integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"pulumi"},
		Quick:        true,
		StackName:    "diffstack",
		ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
			assert.NotNil(t, checkpoint.Latest)
			assert.Equal(t, 5, len(checkpoint.Latest.Resources))
			stackRes := checkpoint.Latest.Resources[0]
			assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
			a := checkpoint.Latest.Resources[1]
			assert.Equal(t, "a", string(a.URN.Name()))
			b := checkpoint.Latest.Resources[2]
			assert.Equal(t, "b", string(b.URN.Name()))
			c := checkpoint.Latest.Resources[3]
			assert.Equal(t, "c", string(c.URN.Name()))
			d := checkpoint.Latest.Resources[4]
			assert.Equal(t, "d", string(d.URN.Name()))
		},
	}

	integration.TestLifeCycleInitAndDestroy(t, &opts, testPreviewUpdatesAndEdits)
}

func testPreviewUpdatesAndEdits(t *testing.T, opts *integration.ProgramTestOptions, dir string) string {
	return integration.TestPreviewAndUpdates(t, opts, dir, testEdits)
}

type EditDirWithValidation struct {
	*integration.EditDir
	Expected string
}

func testEdits(t *testing.T, opts *integration.ProgramTestOptions, dir string) string {
	var edits = []EditDirWithValidation{
		{
			&integration.EditDir{
				Dir:      "step2",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
					assert.NotNil(t, checkpoint.Latest)
					assert.Equal(t, 5, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					b := checkpoint.Latest.Resources[2]
					assert.Equal(t, "b", string(b.URN.Name()))
					c := checkpoint.Latest.Resources[3]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := checkpoint.Latest.Resources[4]
					assert.Equal(t, "e", string(e.URN.Name()))
				},
			},
			`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:diffstack::steps::pulumi:pulumi:Stack::steps-diffstack]</unchanged>
    <added>+ pulumi-nodejs:dynamic:Resource: (create)
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::e]
        __provider: "exports.handler = __d1295c56b890ca4312c6b6aec1efc37f1270220f;\n\nfunction __d1295c56b890ca4312c6b6aec1efc37f1270220f() {\n  return (function() {\n    with({ provider: { diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 } }) {\n\nreturn (() => provider)\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        }))\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __492fe142c8be132f2ccfdc443ed720d77b1ef3a6() {\n  return (function() {\n    with({  }) {\n\nreturn (function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n})\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6, currentID: 0 }) {\n\nreturn ((inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __599534012ff37f9801d962f9c6059b4bc0778921() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\n"</added>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::d]
        __provider: "exports.handler = __d1295c56b890ca4312c6b6aec1efc37f1270220f;\n\nfunction __d1295c56b890ca4312c6b6aec1efc37f1270220f() {\n  return (function() {\n    with({ provider: { diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 } }) {\n\nreturn (() => provider)\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        }))\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __492fe142c8be132f2ccfdc443ed720d77b1ef3a6() {\n  return (function() {\n    with({  }) {\n\nreturn (function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n})\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6, currentID: 0 }) {\n\nreturn ((inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __599534012ff37f9801d962f9c6059b4bc0778921() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\n"</removed>
<info>info</info>: 2 changes performed:
    <added>+ 1 resource created</added>
    <removed>- 1 resource deleted</removed>
      4 resources unchanged`,
		},
		{
			&integration.EditDir{
				Dir:      "step3",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
					assert.NotNil(t, checkpoint.Latest)
					assert.Equal(t, 4, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := checkpoint.Latest.Resources[2]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := checkpoint.Latest.Resources[3]
					assert.Equal(t, "e", string(e.URN.Name()))
				},
			},
			`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:diffstack::steps::pulumi:pulumi:Stack::steps-diffstack]</unchanged>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::b]
        __provider: "exports.handler = __d1295c56b890ca4312c6b6aec1efc37f1270220f;\n\nfunction __d1295c56b890ca4312c6b6aec1efc37f1270220f() {\n  return (function() {\n    with({ provider: { diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 } }) {\n\nreturn (() => provider)\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        }))\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __492fe142c8be132f2ccfdc443ed720d77b1ef3a6() {\n  return (function() {\n    with({  }) {\n\nreturn (function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n})\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6, currentID: 0 }) {\n\nreturn ((inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __599534012ff37f9801d962f9c6059b4bc0778921() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\n"</removed>
<info>info</info>: 1 change performed:
    <removed>- 1 resource deleted</removed>
      4 resources unchanged`,
		},
		{
			&integration.EditDir{
				Dir:      "step4",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
					assert.NotNil(t, checkpoint.Latest)
					// assert.Equal(t, 5, len(checkpoint.Latest.Resources))
					assert.Equal(t, 4, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := checkpoint.Latest.Resources[2]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := checkpoint.Latest.Resources[3]
					assert.Equal(t, "e", string(e.URN.Name()))
					// aPendingDelete := checkpoint.Latest.Resources[4]
					// assert.Equal(t, "a", string(aPendingDelete.URN.Name()))
					// assert.True(t, aPendingDelete.Delete)
				},
			},
			`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:diffstack::steps::pulumi:pulumi:Stack::steps-diffstack]</unchanged>
<info>info</info>: no changes required:`,
		},
		{
			&integration.EditDir{
				Dir:      "step5",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
					assert.NotNil(t, checkpoint.Latest)
					assert.Equal(t, 1, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
				},
			},
			`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:diffstack::steps::pulumi:pulumi:Stack::steps-diffstack]</unchanged>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::e]
        __provider: "exports.handler = __d1295c56b890ca4312c6b6aec1efc37f1270220f;\n\nfunction __d1295c56b890ca4312c6b6aec1efc37f1270220f() {\n  return (function() {\n    with({ provider: { diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 } }) {\n\nreturn (() => provider)\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        }))\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __492fe142c8be132f2ccfdc443ed720d77b1ef3a6() {\n  return (function() {\n    with({  }) {\n\nreturn (function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n})\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6, currentID: 0 }) {\n\nreturn ((inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __599534012ff37f9801d962f9c6059b4bc0778921() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\n"</removed>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::c]
        __provider: "exports.handler = __d1295c56b890ca4312c6b6aec1efc37f1270220f;\n\nfunction __d1295c56b890ca4312c6b6aec1efc37f1270220f() {\n  return (function() {\n    with({ provider: { diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 } }) {\n\nreturn (() => provider)\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        }))\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __492fe142c8be132f2ccfdc443ed720d77b1ef3a6() {\n  return (function() {\n    with({  }) {\n\nreturn (function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n})\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6, currentID: 0 }) {\n\nreturn ((inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __599534012ff37f9801d962f9c6059b4bc0778921() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\n"</removed>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::a]
        __provider: "exports.handler = __d1295c56b890ca4312c6b6aec1efc37f1270220f;\n\nfunction __d1295c56b890ca4312c6b6aec1efc37f1270220f() {\n  return (function() {\n    with({ provider: { diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 } }) {\n\nreturn (() => provider)\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        }))\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __492fe142c8be132f2ccfdc443ed720d77b1ef3a6() {\n  return (function() {\n    with({  }) {\n\nreturn (function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n})\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6, currentID: 0 }) {\n\nreturn ((inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __599534012ff37f9801d962f9c6059b4bc0778921() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\nfunction __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543() {\n  return (function() {\n    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {\n\nreturn ((id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        }))\n\n    }\n  }).apply({ diff: __effb2ddb97a3dfc121870990c86c6ee8f7b1dc44, create: __e47b2d874cf3cf54cd5a54e8e0cf1c8a4a3a25e9, update: __599534012ff37f9801d962f9c6059b4bc0778921, delete: __6c325c14b9ed07e0c974ca9a339ed96d2b5dd543 }, undefined).apply(this, arguments);\n}\n\n"</removed>
<info>info</info>: 3 changes performed:
    <removed>- 3 resources deleted</removed>
      1 resource unchanged`,
		},
	}

	for i, edit := range edits {
		dir = testEdit(t, opts, dir, i, edit)
	}

	return dir
}

func testEdit(t *testing.T, opts *integration.ProgramTestOptions, dir string, i int, edit EditDirWithValidation) string {
	var err error
	dir, err = integration.PrepareProject(t, opts, edit.Dir, dir, edit.Additive)
	if !assert.NoError(t, err, "Expected to apply edit %v atop %v, but got an error %v", edit, dir, err) {
		return dir
	}

	var buf bytes.Buffer

	var oldStdOut = opts.Stdout
	opts.Stdout = &buf
	opts.Verbose = true

	defer func() {
		opts.Stdout = oldStdOut
		opts.Verbose = false
	}()
	if err = integration.PreviewAndUpdate(t, opts, dir, fmt.Sprintf("edit-%d", i)); err != nil {
		return dir
	}

	// Now convert all the color control sequences over to a simpler form for test baseline purposes.
	actual := convertControlSequences(buf.String())

	if edit.Expected != actual {
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(edit.Expected),
			B:        difflib.SplitLines(actual),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  0,
		})

		assert.Fail(t, "Difference between expected and actual:\n"+diff)
	}

	return dir
}

type ColorEnum string

const (
	Clear     ColorEnum = "clear"
	Unchanged ColorEnum = "unchanged"
	Added     ColorEnum = "added"
	Removed   ColorEnum = "removed"
	Info      ColorEnum = "info"
)

// convertControlSequences takes in the output of the pulumi update command (including color sequences)
// and converts it to a simpler form that is easier to baseline.  Control sequences,
// like '<{%fg 2%}}', are converted to simpler code like <added>, with reset controls closing those
// tags.
//
// It's a lot of string munging, but makes it much easier to baseline and validate update diffs.
func convertControlSequences(text string) string {
	getColor := func(startInclusive, endExclusive int) ColorEnum {
		switch text[startInclusive:endExclusive] {
		case "<{%reset%}>":
			return Clear
		case "<{%fg 1%}>":
			return Removed
		case "<{%fg 2%}>":
			return Added
		case "<{%fg 5%}>":
			return Info
		case "<{%fg 8%}>":
			return Unchanged
		default:
			panic("Unexpected match: " + text[startInclusive:endExclusive])
		}
	}

	allWhitespace := func(startInclusive, endExclusive int) bool {
		for i := startInclusive; i < endExclusive; i++ {
			if !unicode.IsSpace(rune(text[i])) {
				return false
			}
		}

		return true
	}

	// Normalize all \r\n to \n's.  it makes all the string processing we need to do much simpler.
	text = strings.Replace(text, "\r\n", "\n", -1)

	// remove the last line.  it contains the duration of the command and can't be tested.s
	lines := strings.Split(text, "\n")
	text = strings.Join(lines[1:len(lines)-2], "\n")

	var result bytes.Buffer
	currentColor := Clear
	index := 0

	anyTagRegex := regexp.MustCompile(`<\{.*?\}>`)
	allTagStartEndPairs := anyTagRegex.FindAllStringIndex(text, -1)

	for pairIndex, startEndPair := range allTagStartEndPairs {
		startInclusive := startEndPair[0]
		endExclusive := startEndPair[1]

		if startInclusive > index {
			result.WriteString(text[index:startInclusive])
		}

		nextColor := getColor(startInclusive, endExclusive)

		index = endExclusive
		if nextColor == currentColor {
			// Ignore it if we see two of the same color in a row.
			continue
		}

		if nextColor == Clear {
			// ignore a clear if it's just followed by whitespace, and then a switch back to
			// our current color.  i.e. something of the form:  <Add> ... <Clear> whitespace <Add> ...
			if pairIndex+1 < len(allTagStartEndPairs) {
				nextNextPair := allTagStartEndPairs[pairIndex+1]
				nextNextColor := getColor(nextNextPair[0], nextNextPair[1])

				if nextNextColor == currentColor {
					if allWhitespace(endExclusive, nextNextPair[0]) {
						continue
					}
				}
			}

			result.WriteString("</" + string(currentColor) + ">")
		} else {
			result.WriteString("<" + string(nextColor) + ">")
		}

		currentColor = nextColor
	}

	if index < len(text) {
		result.WriteString(text[index:])
	}

	if currentColor != Clear {
		result.WriteString("</" + string(currentColor) + ">")
	}

	taggedString := result.String()

	// We'll routinely end up with a line, followed by a newline, followed by and endtag (due to
	// reset chars being written after lines are written).  To make this cleaner in the baseline
	// swap the two so the line ends with the endtag and then is followed by the newline.s
	r, _ := regexp.Compile(`(\n)(\<\/[a-z]+\>)`)
	replacedString := r.ReplaceAllString(taggedString, "$2$1")

	return replacedString
}
