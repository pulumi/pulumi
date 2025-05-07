# Copilot Instructions for Pulumi Repositories

This file provides guidance to GitHub Copilot when working with code in Pulumi repositories.

## Repository Context

This is a central repository for AI tooling configurations and cross-team contexts for Pulumi projects. This repository uses a DRY approach with central data files:

- `/data/teams.json`: Source of truth for team/repository relationships
- `/data/pulumi-context.json`: Centralized Pulumi platform information

## Working Process

When addressing tasks in this repository, always follow this process:

1. Break down the task into a TODO list before starting
2. Complete items one-by-one
3. Verify your work after completion

## What is Pulumi?

Pulumi is an open-source Infrastructure as Code (IaC) platform that enables developers to define, deploy, and manage cloud infrastructure using standard programming languages instead of domain-specific languages or templates.

For complete information on Pulumi's capabilities, architecture, and concepts, refer to `/data/pulumi-context.json`.

## Pulumi MCP Server Integration

Always use the Pulumi MCP server when available:

```json
"mcpServers": {
  "pulumi": {
    "command": "npx",
    "args": ["@pulumi/mcp-server@latest"]
  }
}
```

## Development Guidelines

- Follow Go best practices for Go code
- Use TypeScript for web interfaces and language SDKs
- Ensure provider implementations follow the Pulumi resource model
- Maintain consistent documentation across AI tool configurations
- Run typechecking after code changes
- Prefer running single tests over the entire test suite

## Code Patterns for Pulumi Infrastructure

When suggesting code completions for Pulumi infrastructure, follow these patterns:

### TypeScript/JavaScript
```typescript
// Resource declaration
const bucket = new aws.s3.Bucket("mybucket", {
    // Resource properties
});

// Component resource
class MyInfrastructure extends pulumi.ComponentResource {
    constructor(name: string, args: MyInfrastructureArgs, opts?: pulumi.ComponentResourceOptions) {
        super("pkg:index:MyInfrastructure", name, args, opts);
        // Create child resources
        this.registerOutputs({});
    }
}

// Working with outputs
bucket.id.apply(id => {
    // Use the ID once available
});
```

### Python
```python
# Resource declaration
bucket = aws.s3.Bucket("mybucket", 
    # Resource properties
)

# Component resource
class MyInfrastructure(pulumi.ComponentResource):
    def __init__(self, name, args, opts=None):
        super().__init__('pkg:index:MyInfrastructure', name, None, opts)
        # Create child resources
        self.register_outputs({})

# Working with outputs
bucket.id.apply(lambda id: 
    # Use the ID once available
)
```

### Go
```go
// Resource declaration
bucket, err := s3.NewBucket(ctx, "mybucket", &s3.BucketArgs{
    // Resource properties
})

// Component resource
type MyInfrastructure struct {
    pulumi.ComponentResource
}

func NewMyInfrastructure(ctx *pulumi.Context, name string, args *MyInfrastructureArgs, opts ...pulumi.ResourceOption) (*MyInfrastructure, error) {
    comp := &MyInfrastructure{}
    err := ctx.RegisterComponentResource("pkg:index:MyInfrastructure", name, comp, opts...)
    if err != nil {
        return nil, err
    }
    // Create child resources
    return comp, nil
}

// Working with outputs
bucket.ID().ApplyT(func(id string) interface{} {
    // Use the ID once available
    return nil
})
```

## Synchronization

This repository contains reference configurations that are synchronized to other repositories. Maintain compatibility with the synchronization workflow.