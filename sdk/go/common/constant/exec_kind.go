package constant

// ExecKindAutoLocal is a flag used to identify a command as originating
// from automation API using a traditional Pulumi project.
const ExecKindAutoLocal = "auto.local"

// ExecKindAutoInline is a flag used to identify a command as originating
// from automation API using an inline Pulumi project.
const ExecKindAutoInline = "auto.inline"

// ExecKindCLI is a flag used to identify a command as originating
// from the CLI using a traditional Pulumi project.
const ExecKindCLI = "cli"

// ExitStatusLoggedError is the exit status to indicate that a pulumi program
// has failed, but successfully logged an error message to the engine
const ExitStatusLoggedError = 32
