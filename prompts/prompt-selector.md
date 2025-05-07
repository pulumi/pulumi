# Pulumi Team-Role Prompt Selector

This guide helps you select the appropriate team-role prompt for your current context. Follow these steps to identify which prompt will be most helpful for your task.

## Step 1: Identify Your Team

Which team's codebase are you working with?

- **IAC-Core**: Core Pulumi engine, runtime, and CLI tools
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

- **Service**: Backend service for Pulumi state and team management
  - Primary repository: github.com/pulumi/service

- **Docs**: Documentation and examples
  - Docs: github.com/pulumi/docs
  - Examples: github.com/pulumi/examples

- **Registry**: Pulumi Registry
  - Repository: github.com/pulumi/registry

## Step 2: Identify Your Role

What is your primary role or the task you're working on?

- **Developer**: Implementing features, fixing bugs, writing tests
- **Provider Developer**: Creating and maintaining provider resources
- **Infrastructure Architect**: Designing reusable components and patterns
- **DevOps Engineer**: Automation, deployment, monitoring
- **Documentation Writer**: Creating docs, examples, tutorials
- **Customer Success Engineer**: Helping customers with implementation
- **Community Advocate**: Community engagement and content

## Step 3: Find Your Prompt

Based on your team and role combination, refer to the appropriate prompt file:

```
/prompts/{team}/{role}.md
```

### Available Team-Role Prompts

#### IAC-Core Team
- [Developer](/prompts/iac-core/developer.md): For engineers working on the core Pulumi engine and CLI

#### Providers Team
- [Provider Developer](/prompts/providers/provider_developer.md): For engineers creating and maintaining providers

#### ESC Team
- [Developer](/prompts/esc/developer.md): For engineers working on the ESC framework

#### Service Team
- [DevOps Engineer](/prompts/service/devops_engineer.md): For engineers automating and monitoring service infrastructure
- [Customer Success Engineer](/prompts/service/customer_success_engineer.md): For engineers helping customers implement Pulumi

#### Docs Team
- [Documentation Writer](/prompts/docs/documentation_writer.md): For content creators developing documentation
- [Community Advocate](/prompts/docs/community_advocate.md): For community engagement and content creation

#### Registry Team
- [Infrastructure Architect](/prompts/registry/infrastructure_architect.md): For architects designing registry systems

## Need a New Prompt?

If you don't see a prompt for your specific team-role combination, you can:

1. Use the most closely related existing prompt
2. Create a new prompt following the template in [prompts/README.md](/prompts/README.md)
3. Submit a PR to add the new prompt for others to use