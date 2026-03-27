job "build" {
  input_type = "bool"
  expr       = "invert"

  step "invert" {
    expr = "!input"
  }
}
