config configLexicalName bool {
  __logicalName = "cC-Charlie_charlie.😃⁉️"
}

resource resourceLexicalName "simple:index:Resource" {
  // not necessarily a valid logical name, just testing that it passes through to codegen unmodified
  __logicalName = "aA-Alpha_alpha.🤯⁉️"

  value = configLexicalName
}

output outputLexicalName {
  // Deprecated format for output logical name
  __logicalName = "bB-Beta_beta.💜⁉"
  value = resourceLexicalName.value
}

// New format for output logical name because outputs don't have separate logical names. Even nodejs which just
// does "export" normally for outputs needs that export _to be_ the output name and so if the "logical name"
// isn't a valid nodejs export we have to output it differently.
output "dD-Delta_delta.🔥⁉" {
  value = resourceLexicalName.value
}
