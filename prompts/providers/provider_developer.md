# Provider Developer Prompt

## Role Context
As a provider developer, you create and maintain Pulumi resource providers that map cloud service APIs to Pulumi resources. Your work enables users to manage cloud resources with Pulumi in a consistent, strongly-typed way.

## Key Repositories
Provider repositories follow a standard pattern:
- [pulumi/pulumi-aws](https://github.com/pulumi/pulumi-aws): AWS provider
- [pulumi/pulumi-azure-native](https://github.com/pulumi/pulumi-azure-native): Azure Native provider
- [pulumi/pulumi-gcp](https://github.com/pulumi/pulumi-gcp): Google Cloud provider

## Common Tasks

### Implementing a New Resource
When adding a new resource to a provider:
1. Define the resource in the provider schema
2. Implement CRUD operations mapping to cloud API calls
3. Handle state mapping and diffing
4. Write comprehensive tests
5. Document the resource and its properties
6. Add examples showing resource usage

### Updating Resource API Version
When updating a resource to a new API version:
1. Update the schema to reflect new properties
2. Implement migration logic for state handling
3. Maintain backwards compatibility where possible
4. Update documentation to reflect changes
5. Add examples showing new features

### Fixing Provider Bugs
When fixing provider bugs:
1. Create a minimal reproduction case
2. Identify root cause (schema issue, state handling, API mapping)
3. Implement and test fix
4. Add regression tests
5. Consider backporting to previous versions

## Code Patterns

### Schema Definition
```typescript
// Resource schema definition
resources: {
    "aws:s3/bucket:Bucket": {
        description: "An S3 bucket resource",
        properties: {
            bucket: {
                type: "string",
                description: "The name of the bucket"
            },
            // Additional properties
        },
        // Required properties
        required: ["bucket"]
    }
}
```

### Resource Implementation (Go)
```go
// Resource CRUD operations
func createBucket(ctx context.Context, inputs provider.CreateInputs) (provider.CreateResult, error) {
    // Parse inputs
    // Call cloud API
    // Map API response to outputs
    // Return state
}
```

## Common Pitfalls
- State handling is critical; ensure state is correctly mapped
- API changes can break existing deployments
- Resource naming conventions must be consistent
- Outputs should be properly mapped from API responses
- Error handling must be comprehensive

## Useful Resources
- [Provider Development Guide](https://github.com/pulumi/pulumi/blob/master/provider/README.md)
- [Schema Definition Reference](https://github.com/pulumi/pulumi/blob/master/pkg/codegen/schema/schema.go)
- [Testing Framework Documentation](https://github.com/pulumi/pulumi/blob/master/pkg/testing/README.md)

## Checklist for Pull Requests
- [ ] Schema correctly defines all resource properties
- [ ] CRUD operations implemented correctly
- [ ] State mapping handles all properties
- [ ] Unit tests cover all operations
- [ ] Integration tests with actual cloud resources
- [ ] Documentation updated for all changes
- [ ] Examples added/updated
- [ ] Breaking changes clearly marked