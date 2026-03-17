# Code Generation

## Pipeline Overview

```
Schema (JSON/YAML)
  → schema.Package (pkg/codegen/schema/)
    → Language Generator (go/, python/, nodejs/, dotnet/)
      → map[string][]byte (in-memory filesystem)
```

All generators share the same entry signature:
```go
func GeneratePackage(tool string, pkg *schema.Package, ...) (map[string][]byte, error)
```

Returns a map of file paths to file contents.

## Schema System

`schema/schema.go` defines the type system:
- Primitives: `BoolType`, `IntType`, `NumberType`, `StringType`, `ArchiveType`, `AssetType`, `JSONType`, `AnyType`
- Complex: `MapType`, `ArrayType`, `OptionalType`, `ObjectType`, `EnumType`
- Resources: `Resource` (InputProperties, Properties, Methods)
- Functions: `Function` (Inputs, MultiOutputs)

**Loading:** `schema/loader.go` provides `Loader` and `ReferenceLoader` interfaces.
- `NewPluginLoader(host)` — loads from plugins
- `NewCachedLoader(loader)` — in-memory caching decorator

## Language Generators

Each language has a `modContext` struct organizing types/resources/functions into modules:

| Language | Generator | Program Gen | Lines |
|----------|-----------|-------------|-------|
| Go | `go/gen.go` | `go/gen_program.go` | ~4800 + ~2200 |
| Python | `python/gen.go` | `python/gen_program.go` | ~3300 + ~1700 |
| Node.js | `nodejs/gen.go` | `nodejs/gen_program.go` | ~2900 + ~1200 |
| C#/.NET | `dotnet/gen.go` | `dotnet/gen_program.go` | ~2600 + ~1600 |

**Language-specific config** imported via `schema.Language` interface:
- Go: `GoPackageInfo` — `ImportBasePath`, `Generics` ("none"/"side-by-side"/"generics-only"), etc.
- Python: `PackageInfo` — `PackageName`, `InputTypes` ("classes"/"classes-and-dicts")
- C#: `CSharpPackageInfo` — `RootNamespace`, `Namespaces` map

## PCL (Pulumi Configuration Language)

`hcl2/` + `pcl/` transform HCL syntax into semantic nodes:

```
HCL source → syntax/ (parse) → model/ (type check) → pcl/ (bind to schema) → Program
```

PCL `Program.Nodes` contains: `Config`, `Locals`, `Resource`, `Component`, `Output`.

Each language implements `GenerateProgram(program *pcl.Program)` to convert PCL to code.

## Shared Interfaces

**`ExpressionGenerator`** (`hcl2/model/format/gen.go:28`): Each language implements `Gen*Expression` methods for all expression types (binary ops, function calls, literals, traversals, etc.).

**`DocLanguageHelper`** (`docs.go`): Language-specific documentation helpers for type names, resource names, doc links.

**`Formatter`** (`hcl2/model/format/gen.go:65`): Shared indentation and output utilities.

## Testing

**Golden file tests** in `testing/test/testdata/<test-name>/`:
```
testdata/<test-name>/
  schema.json              # Pulumi schema (if SDK test)
  <test-name>.pp           # PCL source (if program test)
  go/
    codegen-manifest.json  # Expected generation config
    example/               # Expected Go output
  python/
    ...
```

Tests compare generated output against golden files. Split into batches for CI:
- `go/gen_program_test/batch1/`, `batch2/`, ...
- `nodejs/gen_program_test/batch1/`, ...

**Run codegen tests:**
```bash
cd pkg && go test ./codegen/go/...
cd pkg && go test ./codegen/python/...
```

## Common Pitfalls

- **Name collisions**: Schema tokens (`pkg:module/nested:TypeName`) map to language-specific names. Each language tracks `schemaNames` vs `names` with collision detection.
- **Type variants**: `typeDetails` tracks which input/output variants are needed — don't generate unused ones.
- **Circular references**: Schema types can be recursive. Generators must handle unbounded recursion.
- **External packages**: Resources may reference types from other packages. Each generator caches external package contexts (Go uses `globalCache` with mutex).
- **Temp variables**: Complex expressions need intermediates. Each language has a "spiller" pattern (`spills`, `jsonTempSpiller`).
