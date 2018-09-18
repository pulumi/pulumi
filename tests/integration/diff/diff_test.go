// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"unicode"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/stretchr/testify/assert"
)

// TestDiffs tests many combinations of creates, updates, deletes, replacements, and checks the
// output of the command against an expected baseline.
func TestDiffs(t *testing.T) {
	var buf bytes.Buffer

	opts := integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		UpdateCommandlineFlags: []string{"--color=raw", "--non-interactive", "--diff"},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Deployment)
			assert.Equal(t, 6, len(stack.Deployment.Resources))
			stackRes := stack.Deployment.Resources[0]
			assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
			providerRes := stack.Deployment.Resources[1]
			assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
			a := stack.Deployment.Resources[2]
			assert.Equal(t, "a", string(a.URN.Name()))
			b := stack.Deployment.Resources[3]
			assert.Equal(t, "b", string(b.URN.Name()))
			c := stack.Deployment.Resources[4]
			assert.Equal(t, "c", string(c.URN.Name()))
			d := stack.Deployment.Resources[5]
			assert.Equal(t, "d", string(d.URN.Name()))
		},
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
				Stdout:   &buf,
				Verbose:  true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stack.Deployment)
					assert.Equal(t, 6, len(stack.Deployment.Resources))
					stackRes := stack.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stack.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
					a := stack.Deployment.Resources[2]
					assert.Equal(t, "a", string(a.URN.Name()))
					b := stack.Deployment.Resources[3]
					assert.Equal(t, "b", string(b.URN.Name()))
					c := stack.Deployment.Resources[4]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := stack.Deployment.Resources[5]
					assert.Equal(t, "e", string(e.URN.Name()))

					expected :=
						fillStackName(`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:{{.StackName}}::steps::pulumi:pulumi:Stack::steps-{{.StackName}}]</unchanged>
    <changed>~ pulumi-nodejs:dynamic:Resource: (update)
<unchanged>        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::b]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        resource  : "0"</unchanged>
<changed>      ~ state     : </changed><removed>1</removed><changed> => </changed><added>2</added><changed></changed>
    <added>+ pulumi-nodejs:dynamic:Resource: (create)
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::e]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        resource  : "0"
        state     : 1</added>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::d]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        resource  : "0"
        state     : 1</removed>
<info>info</info>: 3 changes performed:
    <added>+ 1 resource created</added>
    <changed>~ 1 resource updated</changed>
    <removed>- 1 resource deleted</removed>
      3 resources unchanged`, stack.StackName)

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
					assert.NotNil(t, stack.Deployment)
					assert.Equal(t, 5, len(stack.Deployment.Resources))
					stackRes := stack.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stack.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
					a := stack.Deployment.Resources[2]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := stack.Deployment.Resources[3]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := stack.Deployment.Resources[4]
					assert.Equal(t, "e", string(e.URN.Name()))

					expected :=
						fillStackName(`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:{{.StackName}}::steps::pulumi:pulumi:Stack::steps-{{.StackName}}]</unchanged>
    <create-replacement>++pulumi-nodejs:dynamic:Resource: (create-replacement)
<unchanged>        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::a]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</unchanged>
<added>      + replace   : 1</added>
<unchanged>        state     : 1</unchanged>
    <replaced>+-pulumi-nodejs:dynamic:Resource: (replace)
<unchanged>        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::a]
      * __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</unchanged>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::b]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        resource  : "0"
        state     : 2</removed>
    <delete-replaced>--pulumi-nodejs:dynamic:Resource: (delete-replaced)
        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::a]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        state     : 1
</delete-replaced><info>info</info>: 2 changes performed:
    <removed>- 1 resource deleted</removed>
    <replaced>+-1 resource replaced</replaced>
      3 resources unchanged`, stack.StackName)

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
					assert.NotNil(t, stack.Deployment)
					assert.Equal(t, 5, len(stack.Deployment.Resources))
					stackRes := stack.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stack.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
					a := stack.Deployment.Resources[2]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := stack.Deployment.Resources[3]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := stack.Deployment.Resources[4]
					assert.Equal(t, "e", string(e.URN.Name()))

					expected := fillStackName(`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:{{.StackName}}::steps::pulumi:pulumi:Stack::steps-{{.StackName}}]</unchanged>
    <create-replacement>++pulumi-nodejs:dynamic:Resource: (create-replacement)
<unchanged>        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::a]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</unchanged>
<changed>      ~ replace   : </changed><removed>1</removed><changed> => </changed><added>2</added><changed></changed>
<unchanged>        state     : 1</unchanged>
    <replaced>+-pulumi-nodejs:dynamic:Resource: (replace)
<unchanged>        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::a]
      * __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"</unchanged>
    <delete-replaced>--pulumi-nodejs:dynamic:Resource: (delete-replaced)
        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::a]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        replace   : 1
        state     : 1
</delete-replaced><info>info</info>: 1 change performed:
    <replaced>+-1 resource replaced</replaced>
      3 resources unchanged`, stack.StackName)

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
					assert.NotNil(t, stack.Deployment)
					assert.Equal(t, 1, len(stack.Deployment.Resources))
					stackRes := stack.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())

					expected := fillStackName(`<unchanged>Performing changes:
* pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:{{.StackName}}::steps::pulumi:pulumi:Stack::steps-{{.StackName}}]</unchanged>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::e]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        resource  : "0"
        state     : 1</removed>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::c]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        resource  : "0"
        state     : 1</removed>
    <removed>- pulumi-nodejs:dynamic:Resource: (delete)
        [id=0]
        [urn=urn:pulumi:{{.StackName}}::steps::pulumi-nodejs:dynamic:Resource::a]
        __provider: "exports.handler = __f0;\n\nvar __provider_proto = {};\n__f1.prototype = __provider_proto;\n__f1.instance = __provider;\nObject.defineProperty(__provider_proto, \"constructor\", { configurable: true, writable: true, value: __f1 });\nObject.defineProperty(__provider_proto, \"diff\", { configurable: true, writable: true, value: __f2 });\nObject.defineProperty(__provider_proto, \"create\", { configurable: true, writable: true, value: __f4 });\nObject.defineProperty(__provider_proto, \"update\", { configurable: true, writable: true, value: __f5 });\nObject.defineProperty(__provider_proto, \"delete\", { configurable: true, writable: true, value: __f6 });\nObject.defineProperty(__provider_proto, \"injectFault\", { configurable: true, writable: true, value: __f7 });\nvar __provider = Object.create(__provider_proto);\n\nfunction __f1() {\n  return (function() {\n    with({  }) {\n\nreturn function /*constructor*/() {\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f3() {\n  return (function() {\n    with({  }) {\n\nreturn function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n};\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f2() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*diff*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            let replaces = [];\n            if (olds.replace !== news.replace) {\n                replaces.push(\"replace\");\n            }\n            return {\n                replaces: replaces,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f4() {\n  return (function() {\n    with({ __awaiter: __f3, currentID: 0 }) {\n\nreturn function /*create*/(inputs) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {\n                id: (currentID++).toString(),\n                outs: undefined,\n            };\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f5() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*update*/(id, olds, news) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n            return {};\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f6() {\n  return (function() {\n    with({ __awaiter: __f3 }) {\n\nreturn function /*delete*/(id, props) {\n        return __awaiter(this, void 0, void 0, function* () {\n            if (this.inject) {\n                throw this.inject;\n            }\n        });\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f7() {\n  return (function() {\n    with({  }) {\n\nreturn function /*injectFault*/(error) {\n        this.inject = error;\n    };\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n\nfunction __f0() {\n  return (function() {\n    with({ provider: __provider }) {\n\nreturn () => provider;\n\n    }\n  }).apply(undefined, undefined).apply(this, arguments);\n}\n"
        replace   : 2
        state     : 1</removed>
<info>info</info>: 3 changes performed:
    <removed>- 3 resources deleted</removed>
      1 resource unchanged`, stack.StackName)

					assertPreviewOutput(t, expected, buf.String())

					buf.Reset()
				},
			},
		},
	}

	integration.ProgramTest(t, &opts)
}

func fillStackName(t string, stackName tokens.QName) string {
	b := bytes.Buffer{}
	template.Must(template.New("").Parse(t)).Execute(&b, struct{ StackName tokens.QName }{StackName: stackName})
	return b.String()
}

func assertPreviewOutput(t *testing.T, expected, outputWithControlSeqeunces string) {
	lines := strings.Split(outputWithControlSeqeunces, "\n")

	// Remove lines from the output that differ across runs. The first two lines of the output are the command line
	// we ran, the second is a message about updating the stack in the cloud, so we drop them.
	lines = lines[2:]

	// The last two lines include a call to stack export and a blank line. Drop them as well.
	lines = lines[:len(lines)-2]

	// If we are connected to a cloud who's URL scheme we recognize, the CLI prints a Permalink for the update, let's
	// drop that (but only if it exists)
	if strings.Index(lines[len(lines)-1], "Permalink: ") != -1 {
		lines = lines[:len(lines)-1]
	}

	// Finally, we have information about how long the update took, which we also drop.
	lines = lines[:len(lines)-1]

	outputWithControlSeqeunces = strings.Join(lines, "\n")
	assertProgramOutput(t, expected, outputWithControlSeqeunces)
}

func assertProgramOutput(t *testing.T, expected, outputWithControlSeqeunces string) {
	// Now convert all the color control sequences over to a simpler form for test baseline purposes.
	actual := convertControlSequences(t, outputWithControlSeqeunces)

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
	Clear             ColorEnum = "clear"
	Unchanged         ColorEnum = "unchanged"
	Added             ColorEnum = "added"
	Removed           ColorEnum = "removed"
	Info              ColorEnum = "info"
	Debug             ColorEnum = "debug"
	Changed           ColorEnum = "changed"
	Replaced          ColorEnum = "replaced"
	CreateReplacement ColorEnum = "create-replacement"
	DeleteReplaced    ColorEnum = "delete-replaced"
	Unknown           ColorEnum = "unknown"
)

// convertControlSequences takes in the output of the pulumi update command (including color sequences)
// and converts it to a simpler form that is easier to baseline.  Control sequences,
// like '<{%fg 2%}}', are converted to simpler code like <added>, with reset controls closing those
// tags.
//
// It's a lot of string munging, but makes it much easier to baseline and validate update diffs.
func convertControlSequences(t *testing.T, text string) string {
	getColor := func(startInclusive, endExclusive int) ColorEnum {
		switch tag := text[startInclusive:endExclusive]; tag {
		case "<{%reset%}>":
			return Clear
		case "<{%fg 1%}>":
			return Removed
		case "<{%fg 2%}>":
			return Added
		case "<{%fg 3%}>":
			return Replaced
		case "<{%fg 13%}>":
			return Replaced
		case "<{%fg 5%}>":
			return Info
		case "<{%fg 8%}>":
			return Unchanged
		case "<{%fg 7%}>":
			return Debug
		case "<{%fg 9%}>":
			return DeleteReplaced
		case "<{%fg 10%}>":
			return CreateReplacement
		case "<{%fg 11%}>":
			return Changed
		default:
			t.Logf("unknown color tag: %v", tag)
			return Unknown
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
