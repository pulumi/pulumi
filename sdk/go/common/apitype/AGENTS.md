# Cloud API types (`sdk/go/common/apitype/`)

These types mirror Pulumi Cloud REST API models and are also consumed by the Pulumi Cloud backend,
so their shape is shared with code outside this repo.

- Don't mark a type here `Deprecated` in favor of a `github.com/pulumi/pulumi-cloud-sdk/go/apitype`
  type, and don't change exported `pkg` signatures to take Cloud SDK types in place of these.
- To migrate a type to the Cloud SDK, replace it with a type alias (`type X = apitype.AppX`, plus
  const aliases for enum values) so existing callers keep compiling. Compare field names, field
  types and JSON tags first; a renamed or retyped field makes the alias a breaking change.
- New code may use Cloud SDK types directly for new endpoints.
- Escalate to a maintainer before migrating or deprecating types here.
