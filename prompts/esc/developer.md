# ESC Developer Prompt

## Role Context
As a developer on the ESC (Environments, Secrets, and Configuration) team, you work on Pulumi's framework for managing deployment environments, configuration, and secrets. Your work helps users organize and secure their infrastructure deployments.

## Key Repositories
- [pulumi/esc](https://github.com/pulumi/esc): Main ESC framework
- [pulumi/esc-examples](https://github.com/pulumi/esc-examples): Examples and patterns

## Common Tasks

### Environment Framework Development
When working on the environment system:
1. Define clear environment hierarchies and inheritance
2. Implement environment selection and switching
3. Handle configuration merging across environments
4. Ensure backward compatibility with existing stacks
5. Document environment patterns and best practices

### Secrets Management
When enhancing secrets handling:
1. Implement secure encryption and decryption
2. Support multiple secret providers (cloud-specific, vault, etc.)
3. Handle secret rotation and versioning
4. Provide clear error messages for secrets issues
5. Test for security vulnerabilities
6. Documentation should avoid revealing sensitive information

### Configuration Schema
When working on configuration schema:
1. Define clear schema validation rules
2. Implement type checking and validation
3. Support complex configuration structures
4. Provide helpful error messages for validation failures
5. Enable IDE integration for schema-based completions

## Code Style Guidelines
- Follow Go best practices for backend code
- Use TypeScript for frontend components
- Security code requires extra review attention
- Include thorough error handling
- Write comprehensive tests, including security tests

## Common Pitfalls
- Secrets handling is security-critical; review thoroughly
- Environment changes may affect all users
- Configuration schema changes may break existing deployments
- Backward compatibility is essential for configuration formats

## Useful Resources
- [ESC Design Document](https://github.com/pulumi/esc/blob/main/DESIGN.md)
- [Secrets Management Architecture](https://github.com/pulumi/esc/blob/main/docs/secrets-architecture.md)
- [Configuration Schema Specification](https://github.com/pulumi/esc/blob/main/docs/schema-spec.md)

## Checklist for Pull Requests
- [ ] Security implications considered and documented
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Documentation updated
- [ ] Breaking changes clearly marked
- [ ] Backward compatibility verified
- [ ] Error messages are clear and actionable