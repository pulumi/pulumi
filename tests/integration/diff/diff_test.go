// Copyright 2017-2018, Pulumi Corporation.
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

package ints

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

// TestDiffs tests many combinations of creates, updates, deletes, replacements, and checks the
// output of the command against an expected baseline.
func TestDiffs(t *testing.T) {
	var buf bytes.Buffer

	opts := integration.ProgramTestOptions{
		Dir:                    "step1",
		Dependencies:           []string{"@pulumi/pulumi"},
		Quick:                  true,
		StackName:              "diffstack",
		UpdateCommandlineFlags: []string{"--color=raw"},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			checkpoint := stack.Checkpoint
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
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
				Stdout:   &buf,
				Verbose:  true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					checkpoint := stack.Checkpoint
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

					expected :=
						`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:diffstack::steps::pulumi:pulumi:Stack::steps-diffstack]</unchanged>
    <added>+ pulumi-nodejs:dynamic:Resource: (create)
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::e]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f3 });\nvar __provider = Object.create(__provider_proto);\n__provider.diff = __f4;\n__provider.create = __f5;\n__provider.update = __f6;\n__provider.delete = __f7;\n\nfunction __f2() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f1() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn function /*constructor*/() {\n        this.diff = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n        this.create = (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n        this.update = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n        this.delete = (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</added>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::d]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f3 });\nvar __provider = Object.create(__provider_proto);\n__provider.diff = __f4;\n__provider.create = __f5;\n__provider.update = __f6;\n__provider.delete = __f7;\n\nfunction __f2() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f1() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn function /*constructor*/() {\n        this.diff = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n        this.create = (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n        this.update = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n        this.delete = (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</removed>
<info>info</info>: 2 changes performed:
    <added>+ 1 resource created</added>
    <removed>- 1 resource deleted</removed>
      4 resources unchanged`

					assertPreviewOutput(t, expected, buf.String())

					buf.Reset()
				},
			},
			{
				Dir:      "step3",
				Additive: true,
				Stdout:   &buf,
				Verbose:  true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					checkpoint := stack.Checkpoint
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

					expected :=
						`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:diffstack::steps::pulumi:pulumi:Stack::steps-diffstack]</unchanged>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::b]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f3 });\nvar __provider = Object.create(__provider_proto);\n__provider.diff = __f4;\n__provider.create = __f5;\n__provider.update = __f6;\n__provider.delete = __f7;\n\nfunction __f2() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f1() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn function /*constructor*/() {\n        this.diff = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n        this.create = (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n        this.update = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n        this.delete = (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</removed>
<info>info</info>: 1 change performed:
    <removed>- 1 resource deleted</removed>
      4 resources unchanged`

					assertPreviewOutput(t, expected, buf.String())

					buf.Reset()
				},
			},
			{
				Dir:      "step4",
				Additive: true,
				Stdout:   &buf,
				Verbose:  true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					checkpoint := stack.Checkpoint
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

					expected := `<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:diffstack::steps::pulumi:pulumi:Stack::steps-diffstack]</unchanged>
<info>info</info>: no changes required:`

					assertPreviewOutput(t, expected, buf.String())

					buf.Reset()
				},
			},
			{
				Dir:      "step5",
				Additive: true,
				Stdout:   &buf,
				Verbose:  true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					checkpoint := stack.Checkpoint
					assert.NotNil(t, checkpoint.Latest)
					assert.Equal(t, 1, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())

					expected := `<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:diffstack::steps::pulumi:pulumi:Stack::steps-diffstack]</unchanged>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::e]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f3 });\nvar __provider = Object.create(__provider_proto);\n__provider.diff = __f4;\n__provider.create = __f5;\n__provider.update = __f6;\n__provider.delete = __f7;\n\nfunction __f2() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f1() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn function /*constructor*/() {\n        this.diff = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n        this.create = (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n        this.update = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n        this.delete = (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</removed>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::c]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f3 });\nvar __provider = Object.create(__provider_proto);\n__provider.diff = __f4;\n__provider.create = __f5;\n__provider.update = __f6;\n__provider.delete = __f7;\n\nfunction __f2() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f1() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn function /*constructor*/() {\n        this.diff = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n        this.create = (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n        this.update = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n        this.delete = (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</removed>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:diffstack::steps::pulumi-nodejs:dynamic:Resource::a]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f3 });\nvar __provider = Object.create(__provider_proto);\n__provider.diff = __f4;\n__provider.create = __f5;\n__provider.update = __f6;\n__provider.delete = __f7;\n\nfunction __f2() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f1() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn function /*constructor*/() {\n        this.diff = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n        this.create = (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n        this.update = (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n        this.delete = (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f2, currentID: 0 }) {\n\nreturn (inputs) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, olds, news) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({ __awaiter: __f2 }) {\n\nreturn (id, props) => __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n\n    }\n  }).apply(__provider, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</removed>
<info>info</info>: 3 changes performed:
    <removed>- 3 resources deleted</removed>
      1 resource unchanged`

					assertPreviewOutput(t, expected, buf.String())

					buf.Reset()
				},
			},
		},
	}

	integration.ProgramTest(t, &opts)
}

func assertPreviewOutput(t *testing.T, expected, outputWithControlSeqeunces string) {
	// Remove the first and last lines.  The first contains the local 	path that the test is running
	// in and last line contains the duration.
	lines := strings.Split(outputWithControlSeqeunces, "\n")
	outputWithControlSeqeunces = strings.Join(lines[1:len(lines)-2], "\n")

	assertProgramOutput(t, expected, outputWithControlSeqeunces)
}

func assertProgramOutput(t *testing.T, expected, outputWithControlSeqeunces string) {
	// Now convert all the color control sequences over to a simpler form for test baseline purposes.
	actual := convertControlSequences(outputWithControlSeqeunces)

	if expected != actual {
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(expected),
			B:        difflib.SplitLines(actual),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  0,
		})

		assert.Fail(t, "Difference between expected and actual:\n"+diff)
	}
}

type ColorEnum string

const (
	Clear     ColorEnum = "clear"
	Unchanged ColorEnum = "unchanged"
	Added     ColorEnum = "added"
	Removed   ColorEnum = "removed"
	Info      ColorEnum = "info"
	Debug     ColorEnum = "debug"
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
		case "<{%fg 7%}>":
			return Debug
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
