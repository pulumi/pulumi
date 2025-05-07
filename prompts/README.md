# Team-Role Specific Prompts

This directory contains specialized prompts for different combinations of teams and roles within Pulumi. These prompts provide contextual information specific to a particular role within a team.

## Directory Structure

Prompts are organized by team and role:

```
prompts/
├── iac-core/
│   └── developer.md
├── providers/
│   └── provider_developer.md
├── esc/
│   └── developer.md
├── service/
│   ├── devops_engineer.md
│   └── customer_success_engineer.md
├── docs/
│   ├── documentation_writer.md
│   └── community_advocate.md
└── registry/
    └── infrastructure_architect.md
```

## Using the Prompts

Each prompt file provides specific guidance for a particular team-role combination, including:

- Role context and primary responsibilities
- Key repositories and their purpose
- Common tasks and workflows with step-by-step guidance
- Code style guidelines specific to that team/role
- Common pitfalls to avoid
- Useful resources and documentation
- PR checklists

## Creating New Prompts

To create a new team-role prompt:

1. Identify the team and role
2. Create a file at `prompts/{team}/{role}.md`
3. Follow the existing prompt format for consistency
4. Include specific, actionable guidance
5. Link to relevant documentation and resources
6. Review with subject matter experts

## Prompt Format

Each prompt should follow this general structure:

```markdown
# Team Role Prompt

## Role Context
Brief description of the role and its importance

## Key Repositories
List of primary repositories with descriptions

## Common Tasks
Step-by-step guidance for frequent tasks

## Code Style Guidelines
Team-specific code style recommendations

## Common Pitfalls
Issues to watch out for

## Useful Resources
Links to documentation and resources

## Checklist for Pull Requests
Items to check before submitting PRs
```

## Reference Data

Prompts can reference the centralized data files for consistent information:

- `/data/teams.json`: Team information and repository relationships
- `/data/roles.json`: Role definitions and responsibilities
- `/data/pulumi-context.json`: Pulumi platform information