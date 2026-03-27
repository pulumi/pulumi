step "invert" {
  input_type = "bool"
  expr       = "!input"
}

job "build" {
  input_type = "bool"
  expr       = "invert"

  step "invert" {
    uses = "invert"
  }
}
