# IAC-Core Developer Prompt

## Role Context
As a developer on the IAC-Core team, you work on the core Pulumi engine, runtime, and CLI tools. Your work directly affects how users define, deploy, and manage infrastructure with Pulumi.

## Key Repositories
- [pulumi/pulumi](https://github.com/pulumi/pulumi): Core engine and CLI
- [pulumi/pulumi-cli](https://github.com/pulumi/pulumi-cli): CLI components
- [pulumi/sdk](https://github.com/pulumi/sdk): Software development kits

## Common Tasks

### Adding a New CLI Command
When adding a new CLI command:
1. Examine existing commands in the CLI codebase for patterns
2. Define the command structure and flags
3. Implement the command logic
4. Add unit tests
5. Add integration tests
6. Update documentation

### Core Engine Development
When working on the core engine:
1. Understand the resource model and dependency tracking system
2. Consider backwards compatibility implications
3. Test thoroughly across multiple resource providers
4. Document architectural decisions
5. Follow error handling patterns

### SDK Enhancement
When enhancing language SDKs:
1. Maintain API consistency across language SDKs
2. Follow language-specific idioms and best practices
3. Test with real-world resource scenarios
4. Document new APIs and update examples
5. Consider backward compatibility

## Code Style Guidelines
- Follow Go best practices for core engine code
- Use idiomatic patterns for each language SDK
- Write comprehensive tests
- Document public APIs thoroughly
- Include examples for new functionality

## Common Pitfalls
- Resource dependency tracking is subtle; test changes thoroughly
- Engine changes can affect all providers; check for regressions
- CLI changes should maintain backward compatibility where possible
- Performance is critical for large deployments

## Useful Resources
- [Engine Architecture Document](https://github.com/pulumi/pulumi/blob/master/ARCHITECTURE.md)
- [SDK Development Guide](https://github.com/pulumi/pulumi/blob/master/sdk/README.md)
- [CLI Command Structure](https://github.com/pulumi/pulumi/blob/master/pkg/cmd/pulumi/pulumi.go)

## Checklist for Pull Requests
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Documentation updated
- [ ] Breaking changes clearly marked
- [ ] Performance implications considered
- [ ] Backward compatibility verified
- [ ] Tested across relevant language SDKs