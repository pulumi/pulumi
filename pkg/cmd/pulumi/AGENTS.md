# CLI Commands

## Command Pattern

Every command follows this structure:

```go
func NewFooCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
    var flagVal string
    cmd := &cobra.Command{
        Use:   "foo",
        Short: "...",
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            opts := display.Options{Color: cmdutil.GetGlobalColorization()}
            // ...
            return nil
        },
    }
    constrictor.AttachArguments(cmd, constrictor.NoArgs)
    cmd.PersistentFlags().StringVarP(&flagVal, "flag", "f", "", "description")
    return cmd
}
```

Key elements:
- Constructor returns `*cobra.Command`, flags declared as local vars in closure
- Context from `cmd.Context()`, display opts from `cmdutil.GetGlobalColorization()`
- Arguments via `constrictor.AttachArguments()` (not Cobra's `Args` field)
- Register in `pulumi.go` → `NewPulumiCmd()` → appropriate `commandGroup`

## Command Registration

Commands are organized into groups in `pulumi.go` (lines 399-501):

| Group | Commands |
|-------|----------|
| Stack Management | `new`, `config`, `stack`, `console`, `import`, `refresh`, `state`, `install` |
| Deployment | `up`, `destroy`, `preview`, `cancel` |
| Environment | `env` |
| Pulumi Cloud | `login`, `logout`, `whoami`, `org`, `project`, `deployment` |
| Policy | `policy` |
| Plugin | `plugin`, `schema`, `package`, `template` |
| Other | `version`, `about`, `completion` |

Experimental/developer commands gated by env vars: `convert`, `watch`, `logs`, `trace`, `events`, `clispec`.

## Accessing Core Services

```go
// Backend (Cloud or DIY)
be, err := cmdBackend.CurrentBackend(ctx, ws, lm, project, opts)

// Stack
s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, lm, stackName, OfferNew, opts)

// Interactive mode
if cmdutil.Interactive() { /* prompt user */ }
```

## Argument Handling (constrictor)

`constrictor/constrictor.go` provides structured argument metadata:

```go
// No arguments
constrictor.AttachArguments(cmd, constrictor.NoArgs)

// Required + optional args
constrictor.AttachArguments(cmd, &constrictor.Arguments{
    Arguments: []constrictor.Argument{{Name: "template-or-url", Usage: "[template|url]"}},
    Required:  0,
    Variadic:  false,
})

// Variadic (e.g., multiple URNs)
constrictor.AttachArguments(cmd, &constrictor.Arguments{
    Arguments: []constrictor.Argument{{Name: "urn"}},
    Required:  1,
    Variadic:  true,
})
```

## Subcommand Pattern

Parent commands share state with children via pointers:

```go
func NewConfigCmd(ws pkgWorkspace.Context) *cobra.Command {
    var stack string
    cmd := &cobra.Command{Use: "config", Short: "Manage configuration"}
    cmd.PersistentFlags().StringVarP(&stack, "stack", "s", "", "stack name")
    cmd.AddCommand(newConfigGetCmd(ws, &stack))
    cmd.AddCommand(newConfigSetCmd(ws, &stack))
    return cmd
}
```

## Key Helper Packages

| Package | Purpose |
|---------|---------|
| `backend/` (local) | `CurrentBackend()`, `IsDIYBackend()`, `LoginManager` |
| `cmd/` | `DisplayErrorMessage()`, `processCmdErrors()` |
| `ui/` | Survey-based prompts, `PromptForValue()` |
| `constrictor/` | Structured argument specs for CLI introspection |
| `stack/` (local) | `RequireStack()`, stack selection utilities |

## Flag Patterns

```go
// Mutually exclusive
cmd.MarkFlagsMutuallyExclusive("target", "exclude")

// Hidden
_ = cmd.PersistentFlags().MarkHidden("tracing-header")

// Deprecated
_ = cmd.PersistentFlags().MarkDeprecated("copilot", "please use --neo instead")
```
