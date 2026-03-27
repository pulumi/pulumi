job "build" {
  input_type = {
    input = bool
  }
  expr       = invert

  step "invert" {
    expr = !input
  }
}
