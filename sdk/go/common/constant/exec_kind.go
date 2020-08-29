package constant

// ExecKindAutoLocal is a flag used to indentify a command as originating
// from automation API using a traditional Pulumi project.
const ExecKindAutoLocal = "auto.local"

// ExecKindAutoInline is a flag used to indentify a command as originating
// from automation API using an inline Pulumi project.
const ExecKindAutoInline = "auto.inline"

// ExecKindCLI is a flag used to indentify a command as originating
// from the CLI using a traditional Pulumi project.
const ExecKindCLI = "cli"
