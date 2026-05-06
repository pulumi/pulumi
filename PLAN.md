# Rapid round-trip test for `GenerateHCL2Definition`

## Goal

A property-based (rapid) test that asserts: for any valid Pulumi schema and any
conforming resource state, running `GenerateHCL2Definition` and then binding /
evaluating the resulting PCL block reproduces the original state.

This is a living document. Update it as steps complete or scope shifts.

## Status

- [x] Step 1 — schema generator (`pkg/codegen/testing/utils/rapidschema`)
- [x] Step 2 — schema-conforming value generators (`pkg/codegen/testing/utils/rapidresource`)
- [x] Step 3 — `resource.State` generator (`pkg/importer/rapid`)
- [x] Step 4 — driver: round-trip via `pcl.BindProgram` + `renderInputs`, assert inputs (with TODO for full PCL evaluator)
- [x] Step 5 — known-failure filters (one per real failure observed during step 4; see list below)
- [ ] Step 6 — assert `pcl.BindProgram` produces no diagnostics (folded into step 4 driver)
- [ ] Step 7 — wire into normal CI (this is true for any test, and doesn't need to be a seperate step)

### Step 5 filters added during step 4 (`pkg/importer/hcl2_rapid_test.go`)

Each is a real production-side issue surfaced by the driver; the filter
is keyed to the bug so the skip can be removed when the bug is fixed.

1. **HCL formatter `\xNN` escape bug.** The model formatter writes
   non-printable bytes as `\xNN`, but the HCL grammar only accepts
   `\uNNNN`. Any input string or map key with a control byte fails
   parse. Skipped at the parse-diagnostics check.
2. **`NYI: archives` / `NYI: assets`.** `generateValue`
   (`pkg/importer/hcl2.go:821`, `:842`) returns these errors instead of
   emitting `fileArchive` / `fileAsset` calls. Skipped on
   `GenerateHCL2Definition` error.
3. **Optional-null normalization.** `generatePropertyValue`
   (`pkg/importer/hcl2.go:588`) treats `null` as "no attribute"
   (optional) or a typed zero (required), so `{a: null}` round-trips to
   `{}` or to `{a: false}`. Recursive: applies inside ObjectType too.
   Skipped via `hasNullInput`.
4. **Unicode NFC normalization.** The HCL parser (or cty) normalizes
   string literals to NFC, so `"À"` (decomposed bytes `41 cc 80`)
   round-trips to `"À"` (precomposed bytes `c3 80`). Skipped via
   `hasNonNFCString`.
5. **`__`-prefixed map keys silently dropped.** `generateValue`'s
   `MapType` / `Any` branch (`pkg/importer/hcl2.go:884`) drops every
   key beginning with `__` when re-emitting a map. The schema does
   not allow `ObjectType` property names to start with `__`, so this
   only fires for `MapType` / `Any`-shaped values. Surfaced once the
   map-key generator was biased to draw `__`-prefixed keys
   (`mapKeyGenerator` in `rapidresource.go`); skipped via
   `hasInternalMapKey`.
6. **`ignoreChanges` emitted as bare identifier.**
   `makeResourceOptions` (`pkg/importer/hcl2.go:322`) emits each entry
   as a `LiteralValueExpression`, which the HCL formatter writes
   verbatim (e.g. `[d]` instead of `["d"]`). The binder then treats
   the entry as a variable reference, which can produce a "circular
   reference" diagnostic when the property name collides with the
   resource or another bound name. Skipped when
   `state.IgnoreChanges` is non-empty. Fix is to emit a
   `TemplateExpression` here like the `import` option does.

### Other step 4 changes

- `pkg/importer/rapid/rapid.go`: satellites are now drawn with a URN
  distinct from the state's URN and the provider's URN (`taken` map
  filter on the URN generator). Without this, a satellite could share
  `state.URN`, producing both invalid snapshots (duplicate URN) and
  self-referential envelopes (e.g. `replaceWith` pointing at the
  resource itself, which the binder rejects as a circular reference).
- `pkg/importer/hcl2_test.go`: `renderResource` and its `render*`
  helpers now take `require.TestingT` instead of `*testing.T` so the
  rapid driver (which receives `*rapid.T`) can call them.
  `renderLiteralValue` learned to handle `cty` null literals (returned
  by the production code as the bound form of `null`).

Each step is self-contained: implement, run the build/lint/tests for the
touched code, then check in with the user before starting the next step.

## Background — what already exists

Captured here so we don't redo discovery:

- `GenerateHCL2Definition` is at `pkg/importer/hcl2.go:81`. Signature:
  `(loader schema.Loader, state *resource.State, importState ImportState) → (*model.Block, *schema.PackageDescriptor, error)`.
  `ImportState` (line 50) holds `Names`, `PathedLiteralValues`, and
  `Snapshot []*resource.State`. `Snapshot` must contain the provider resource
  referenced by `state.Provider`.
- The non-rapid round-trip is `TestGenerateHCL2Definition` at
  `pkg/importer/hcl2_test.go:263`: format block → `syntax.Parser.ParseFile` →
  `pcl.BindProgram` → `renderResource(...)` (line 176) → compare to original
  `state`. `renderResource` reconstructs a `*resource.State` from a bound
  `*pcl.Resource` purely in-process; we reuse this pattern.
- A minimal `schema.Loader` stub exists at
  `pkg/codegen/pcl/binder_resource_test.go:322` (`stubSchemaLoader`). We will
  copy that pattern (or expose an equivalent in the new test package) so the
  test never touches disk.
- `schema.BindSpec(spec, loader, opts)` at `pkg/codegen/schema/bind.go:338`
  turns a `PackageSpec` into a `*schema.Package` with diagnostics. Use this
  (not `ImportSpec`) so we can assert there are zero diagnostics.
- Existing rapid generators (none cover the full job):
  - `sdk/go/common/resource/testing/rapid.go` — unconstrained
    `resource.PropertyValue` / `PropertyMap` / `URN` generators.
  - `sdk/go/property/testing/rapid.go` — same shape for the newer
    `property.Value` type.
  - `pkg/engine/lifecycletest/fuzzing/resource.go` — generates resource
    *envelope* state (`URN`/`ID`/deps/options) but no schema-typed inputs.

## Step 1 — Schema generator

**Location:** new package `pkg/codegen/testing/utils/rapidschema`.

**Public surface:** exactly one exported function (name TBD during
implementation — likely `Package() *rapid.Generator[*schema.Package]`).
Everything else is unexported.

**Behavior:**

- Always generate fully complex schemas. No options struct, no toggles.
  Consumers filter out shapes they don't care about.
- Cover the full breadth of `schema.PackageSpec` / `ResourceSpec` /
  `ComplexTypeSpec` / `PropertySpec` / `TypeSpec` — primitives, arrays, maps,
  named complex objects, enums, unions (with and without discriminators),
  references, optional/required, plain, secret, archives, assets, JSON, Any.
- May generate a schema with no resources, callers will need to filter if they
  want at least one.
- Produce only **valid** schemas by construction — i.e. names are
  well-formed tokens, refs always resolve, required lists only mention
  declared properties, discriminator mappings only target object types in the
  union, etc. Invalid combinations are excluded by the generator, not by
  filtering.
- After generation, bind via `schema.BindSpec`. Assert (inside the generator)
  that bind returns no error and an empty `hcl.Diagnostics`. A bind failure
  is a generator bug — fail the test, do not skip.
- Return the `*schema.Package`. (We surface the bound package, not the spec,
  because every downstream consumer needs the bound form.)

**Definition of done:** the generator runs as a standalone rapid test (`func
TestRapidSchemaGenerator(t *testing.T) { rapid.Check(t, ...) }`) which draws
~hundreds of packages and confirms every one binds cleanly. The test validates
that we have sampled at least one of each expected property (primitive, array,
map, type ref, etc.)

## Step 2 — Schema-conforming value generators

**Location:** new package `pkg/codegen/testing/utils/rapidresource`.

**Public surface:** exactly three exported functions:

```go
func ResourceInputs(r *schema.Resource)     *rapid.Generator[property.Map]
func ResourceProperties(r *schema.Resource) *rapid.Generator[property.Map]
func ResourceState(r *schema.Resource)      *rapid.Generator[*property.Map]
```

Each defers to an internal helper that takes a `[]*schema.Property` (or
equivalent property list). `ResourceState` returns a pointer because some
resources have no state inputs at all — in that case the generator yields
`nil`.

**Behavior:**

- Generate a `property.Value` for every type the schema can carry. Cover the
  full set: `BoolType`, `StringType`, `IntType`, `NumberType`, `ArrayType`,
  `MapType`, `ObjectType`, `UnionType` (pick a branch — for discriminated
  unions, generate a value whose discriminator field matches the chosen
  branch's mapping), `TokenType` (recurse on `UnderlyingType` when set,
  otherwise treat as the underlying primitive), `OptionalType`, `InputType`,
  `ArchiveType`, `AssetType`, `JSONType`, `AnyType`, and enums (sample from
  the enum's value list).
- Required properties are always present; optional properties are sometimes
  absent (and sometimes explicitly null when the schema permits it).
- Output type is `property.Value` (the newer `sdk/go/property` API), since
  that is what `GenerateHCL2Definition` consumes after
  `resource.FromResourcePropertyValue`.

**Definition of done:** standalone rapid test that, for each resource in a
generated package (step 1 fed in), draws inputs/properties/state and asserts
they are structurally typed against their declared schema (re-using
`valueStructurallyTypedAs` from `pkg/importer/hcl2.go:599` is fine for this, but
we need to have that test in pkg/importer/hcl2_test.go since we don't want to
import pkg/importer in our test library.).

## Step 3 — `resource.State` generator

**Location:** new package `pkg/importer/rapid`.

**Public surface:** a function (name TBD — likely
`State(*schema.Package) *rapid.Generator[*resource.State]`). May expose more
helpers as needed (e.g. for the provider snapshot that the driver feeds into
`ImportState.Snapshot`); decide during implementation but keep the public
surface minimal.

**Behavior:**

- Pick a random resource from the schema package.
- Generate that resource's inputs via `rapidresource.ResourceInputs`.
- Generate the rest of the `resource.State` envelope using a fresh rapid
  generator for resource options: `Protect`, `RetainOnDelete`, `IgnoreChanges`,
  `DeletedWith`, `ReplaceWith`, `ImportID`, `Parent`, `Provider`,
  `Dependencies`, `PropertyDependencies`, `Aliases`, etc. Where these refer
  to other URNs (parent, deletedWith, replaceWith, dependencies), the
  generator must also surface a paired snapshot of referenced resources so
  the driver can wire `ImportState.Names`/`ImportState.Snapshot` correctly.
  (Reuse `pkg/engine/lifecycletest/fuzzing/resource.go` shapes where they
  fit; do not depend on that package directly if it would create an import
  cycle — copy the small bits we need.)
- Inputs in the returned state must be `resource.PropertyMap` (use
  `resource.ToResourcePropertyMap`).

**Definition of done:** standalone rapid test that draws a state from a
generated package and confirms the state's `Type` matches a resource declared
in the package and its `Inputs` are structurally typed against that
resource's input properties.

## Step 4 — Driver: round-trip and assert

**Location:** `pkg/importer/hcl2_rapid_test.go`.

**Behavior:**

```go
rapid.Check(t, func(t *rapid.T) {
    pkg   := rapidschema.Package().Draw(t, "pkg")
    state := rapidimporter.State(pkg).Draw(t, "state")        // step 3

    loader := &stubSchemaLoader{Package: pkg}                 // copy of binder_resource_test.go pattern
    importState := buildImportState(state)                    // names + snapshot incl. provider

    block, _, err := GenerateHCL2Definition(loader, state, importState)
    require.NoError(t, err)

    parser := syntax.NewParser()
    require.NoError(t, parser.ParseFile(strings.NewReader(fmt.Sprint(block)), "x.pp"))
    require.False(t, parser.Diagnostics.HasErrors())

    prog, diags, err := pcl.BindProgram(parser.Files, pcl.Loader(loader), pcl.AllowMissingVariables)
    require.NoError(t, err)
    require.False(t, diags.HasErrors())                       // step 6 — fold in here

    got := renderResource(t, prog.Nodes[0].(*pcl.Resource))
    require.True(t, got.Inputs.DeepEquals(state.Inputs))
})
```

We can use the existing package-private `renderResource` because the test
lives in `package importer`.

**TODO (tracked in this plan, not in code unless implementation lands):**
swap `renderResource` for the full `sdk/pcl` evaluator so that we can also
assert on every resource option (`Protect`, `RetainOnDelete`, `IgnoreChanges`,
`DeletedWith`, `ReplaceWith`, `ImportID`, `Parent`, `Provider`,
`Dependencies`, `PropertyDependencies`, `Aliases`). Initial pass asserts only
on `Inputs`; widen as soon as the evaluator is wired in.

## Step 5 — Known-failure filter (lazy)

Do **not** add up-front. Only introduce a filter / `t.Skip` when a concrete
failing case is observed and we have decided we are not going to fix it
right now. Each entry must:

- Be expressed either as `Generator.Filter(...)` (preferred when the failure
  class is detectable up-front) or as a `t.Skip("<reason>")` keyed to a
  specific issue.
- Reference the upstream issue / link in a comment so the skip can be
  removed when fixed.

## Step 6 — Bind diagnostics assertion

Fold `require.False(t, diags.HasErrors())` into the step-4 driver. No
separate work item beyond making sure the assertion is present and that any
diagnostic content is surfaced in the failure message (e.g. via
`diags.Error()`).

## Step 7 — CI

Test runs as part of the normal `make test_fast` / `make test_all` flow —
no build tag, no nightly-only gating. We will pick a default `rapid.Check`
iteration count that keeps wall time reasonable on the importer package; if
that turns out to be too slow we revisit then.

## Notes for the implementer

- Each new package only exposes the documented surface; everything else is
  unexported.
- New files get the standard Pulumi copyright header stamped with the current
  year.
- After every step run `mise exec -- make format && mise exec -- make lint`
  on the touched module(s) and the relevant `go test` invocation.
- Update the **Status** checklist above the moment a step is complete, and
  edit any section whose plan has shifted before starting the next step.
