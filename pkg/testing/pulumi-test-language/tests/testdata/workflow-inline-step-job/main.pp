job "build" {
  input_type = "workflow:index:BoolInput"
  expr       = "invert"

  step "invert" {
    expr = "!input"
  }
}
