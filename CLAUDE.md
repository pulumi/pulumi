# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

This repository serves as a central hub for Pulumi's AI tooling configurations, best practices, and cross-team context. It maintains consistent AI contexts across different Pulumi projects.

## Working Process

When addressing tasks in this repository, ALWAYS follow this process:

1. Break down the task into a TODO list before starting (MANDATORY)
2. When presented with a complex task, use the word "think" to trigger detailed analysis
3. Complete items one-by-one, checking them off as you go
4. Verify your work after completion
5. Preserve TODO list state when sessions are paused to enable resuming tasks

Example TODO list format:
```
TODO:
- [ ] Understand the requirements
- [ ] Plan the approach
- [ ] Implement the solution
- [ ] Test and validate
- [ ] Document any new changes
```

### IMPORTANT: Task Continuity Guidelines

- ALWAYS maintain TODO lists throughout the session
- When a session might be interrupted or paused, clearly state the current progress
- Number each TODO item to make tracking progress easier
- When resuming work, first review the remaining TODO items
- Mark completed items with [x] and keep them visible for context
- For long-running tasks, include a "Current Status" section

Example of resumable TODO list:
```
TODO (Current Status: Implementing validation logic):
- [x] 1. Understand the requirements
- [x] 2. Design the architecture
- [ ] 3. Implement the solution
  - [x] 3.1. Set up project structure
  - [x] 3.2. Implement core functionality
  - [ ] 3.3. Add validation logic <-- CURRENT TASK
  - [ ] 3.4. Add error handling
- [ ] 4. Write tests
- [ ] 5. Document the implementation
```

## Best Practices

### Code Style
- Use ES modules (import/export) syntax, not CommonJS (require)
- Destructure imports when possible (e.g., import { foo } from 'bar')
- Follow language-specific formatting standards:
  - Go: gofmt with standard settings
  - TypeScript: Prettier with standard settings
  - Python: Black formatter
  - C#: .NET formatter

### Quality Assurance
- Run typechecking after making code changes
- Run appropriate linting tools before completing tasks
- Prefer running single tests over the entire test suite for performance
- Test both happy paths and error cases
- Keep error handling consistent with existing patterns

### Source Control
- Check only the minimum files needed
- Prefer descriptive commit messages that explain the why, not just the what
- Add the special signature as required

### Repository Structure

This repository uses a DRY approach with centralized data:

- `/data/teams.json`: Source of truth for team/repository relationships
- `/data/pulumi-context.json`: Centralized Pulumi platform information 
- AI tool configurations reference these central files rather than duplicating

## Pulumi MCP Server

Always use the Pulumi MCP server when available:

```json
"mcpServers": {
  "pulumi": {
    "command": "npx",
    "args": ["@pulumi/mcp-server@latest"]
  }
}
```

## Teams and Roles

### Teams

Reference the `/data/teams.json` file for comprehensive team information. Major teams include:

- **IAC-Core**: Core infrastructure as code engine, runtime, and CLI tools
  - Primary repository: github.com/pulumi/pulumi
  - Additional repositories: github.com/pulumi/pulumi-cli, github.com/pulumi/sdk
  
- **ESC**: Environment, Secrets, and Configuration framework
  - Primary repository: github.com/pulumi/esc
  - Examples: github.com/pulumi/esc-examples
  
- **Providers**: Cloud provider-specific implementations
  - AWS: github.com/pulumi/pulumi-aws
  - Azure: github.com/pulumi/pulumi-azure-native
  - GCP: github.com/pulumi/pulumi-gcp
  - Kubernetes: github.com/pulumi/pulumi-kubernetes
  - Many others (see /data/teams.json for comprehensive list)
  
- **Service**: Backend service for Pulumi state and team management
  - Primary repository: github.com/pulumi/service
  
- **Docs & Examples**: Pulumi documentation and example code
  - Docs: github.com/pulumi/docs
  - Examples: github.com/pulumi/examples

### Roles

Reference the `/data/roles.json` file for detailed role information. Key roles include:

- **Developer**: Engineers who write and maintain Pulumi code
- **Provider Developer**: Engineers who create and maintain Pulumi providers
- **Infrastructure Architect**: Professionals who design cloud infrastructure patterns
- **DevOps Engineer**: Engineers focused on deployment and operations
- **Documentation Writer**: Content creators for docs, examples, and tutorials
- **Customer Success Engineer**: Engineers who help customers implement Pulumi
- **Community Advocate**: Professionals who engage with the Pulumi community

### Team-Role Specific Guidance

For targeted guidance based on both team and role, refer to the corresponding prompt in the `/prompts` directory:

```
/prompts/{team}/{role}.md
```

For example:
- `/prompts/iac-core/developer.md` - Guidance for developers on the IAC-Core team
- `/prompts/providers/provider_developer.md` - Guidance for provider developers
- `/prompts/docs/documentation_writer.md` - Guidance for documentation writers

## What is Pulumi?

Pulumi is an open-source Infrastructure as Code (IaC) platform that enables developers to define, deploy, and manage cloud infrastructure using familiar programming languages instead of domain-specific languages or templates.

For detailed information on Pulumi, refer to `/data/pulumi-context.json`.

## Workflow Commands

- Use `go mod tidy` for Go dependency management
- Run `go test ./...` to test Go code
- Follow standard Go formatting with `gofmt -s -w .`
- Use `golangci-lint run` for linting Go code
- Use `npm run typecheck` to check TypeScript types
- Use `npm run lint` for linting JavaScript/TypeScript
- Use `npm run format` to format JavaScript/TypeScript

## Guidelines

- Maintain consistency across AI tool configurations 
- Ensure context files are valid and properly formatted
- Keep provider-specific knowledge in the appropriate context files
- When creating new AI tool configurations, follow the established patterns
- Respect the synchronization system for cross-repository contexts
- Prefer referencing central data files over duplicating information

## Lessons Learned

### DRY Approach for Context Management
- Central data files are the source of truth (`data/teams.json`, `data/roles.json`, etc.)
- AI tool configs should reference central files rather than duplicating information
- This reduces maintenance burden and keeps information consistent

### Hierarchical Role-Team Structure
- Teams can manage multiple repositories (`repositories` array in teams.json)
- Roles can exist across multiple teams but with team-specific responsibilities
- Team-role combinations provide the most targeted guidance (`/prompts/{team}/{role}.md`)

### Task Management Essentials
- TODO lists are MANDATORY for tracking progress and enabling task resumption
- Complex tasks should be broken down into numbered sub-tasks
- Maintain task state when sessions might be interrupted
- The "think" keyword triggers deeper analysis for complex problems

### Context Sync System
- The GitHub Action workflow synchronizes all context files to target repositories
- Updates propagate via pull requests to maintain consistency
- All AI tools should use the same underlying knowledge base# Repository: pulumi/pulumi
