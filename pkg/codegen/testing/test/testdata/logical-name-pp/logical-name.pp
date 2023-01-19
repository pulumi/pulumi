config configLexicalName string {
  __logicalName = "cC-Charlie_charlie.😃⁉️"
}

resource resourceLexicalName "random:index/randomPet:RandomPet" {
  // not necessarily a valid logical name, just testing that it passes through to codegen unmodified
  __logicalName = "aA-Alpha_alpha.🤯⁉️"

  prefix = configLexicalName
}

output outputLexicalName {
  __logicalName = "bB-Beta_beta.💜⁉"
  value = resourceLexicalName.id
}
