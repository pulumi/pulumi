# How To Run a C# script

To run it from a console:

- Install the `dotnet-script` tool: `dotnet tool install -g dotnet-script`
- Build the solution with `PulumiAzure` from the parent folder
- Execute `dotnet script main.csx`

```
    └─ core.ResourceGroup     rg          created
    └─ storage.Account        sa          created
```