# Pulumi Development Workflow Guide

## Getting Started

This guide outlines general development workflow patterns for contributing to the Pulumi project.

## Development Workflow

### 1. Understanding the Codebase Structure

Pulumi CLI commands follow a consistent structure:
- Main commands are organized in `pkg/cmd/pulumi/`
- Subcommands are often grouped in subdirectories 
- Each command typically has its own file
- Test files follow the pattern `filename_test.go`

### 2. Code Quality

1. Linting is enforced via CI:
   - Run `make lint` to check for issues
   - Run `make lint_fix` to automatically fix some issues
   - Common issues include:
     - Unchecked error returns from IO operations
     - Pre-allocation of slices
     - Unused function results (especially append)

2. Common patterns:
   - Handle nil values properly
   - Extract duplicated logic into helper functions
   - Add clear comments explaining non-obvious logic
   - Use structured error handling with `fmt.Errorf("context: %w", err)`

### 3. Pull Request Process

1. Create a branch for your work
2. Commit your changes with descriptive messages
3. Add a changelog entry in `changelog/pending/` directory
4. Create a PR with a clear description of changes
5. Address feedback from reviewers
6. Ensure CI checks pass before merging

## Common Tasks

### Running Tests

```bash
# Run tests for a specific package
go test -v ./path/to/package/...
```

### Linting

```bash
# Check for linting issues
make lint

# Automatically fix some linting issues
make lint_fix
```

### Changelog Entries

Use the `make changelog` command to create a new changelog entry:

```bash
make changelog
```

This will prompt you for the necessary information and create a file in `changelog/pending/` with the format `YYYYMMDD--component--short-description.yaml` containing:

```yaml
feature: |
  Description of the new feature.

component: component-name
pr: #pr-number
```

Components can be various parts of the system (cli, sdk, etc.)

## Tips and Best Practices

1. **Code organization**:
   - Keep related functionality together
   - Use descriptive variable and function names
   - Follow existing patterns in the codebase

2. **Error handling**:
   - Check all error returns, especially from IO operations
   - Provide context in error messages
   - Use appropriate error handling patterns (e.g., `fmt.Errorf("context: %w", err)`)

3. **Testing**:
   - Mock external dependencies
   - Test edge cases and error conditions
   - For CLI commands, use dependency injection to make testing easier
   - Use buffer capture for testing command output instead of capturing stdout directly
   - Disable parallel tests (`//nolint:paralleltest`) when they manipulate shared resources

4. **Code reuse**:
   - Extract common functionality into helper functions
   - Avoid duplicating logic
   - Use existing utilities in the codebase

5. **User experience**:
   - Provide helpful error messages
   - Include examples in help text
   - Support sensible flags for user convenience

6. **Console Output & Testability**:
   - Use an `io.Writer` field (e.g., `Stdout`) in command structs instead of writing directly to `os.Stdout`
   - Initialize writer field with `os.Stdout` by default in production code
   - Use `bytes.Buffer` in tests to capture and verify output
   - This pattern improves testability and avoids data races in tests

7. **Data Race Prevention**:
   - Be cautious with global resources like stdout when writing tests
   - Use separate buffers for capturing output in parallel tests
   - If tests manipulate stdout, consider disabling parallel execution
   - Add clear comments to explain why parallelism is disabled
