step "invert" {
  input_type = "workflow:index:BoolInput"
  expr       = "!input"
}

job "build" {
  input_type = "workflow:index:BoolInput"
  expr       = "invert"

  step "invert" {
    uses = "invert"
  }
}
