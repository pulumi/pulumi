step "invert" {
  input_type = {
    input = bool
  }
  expr       = !args.input
}

job "build" {
  input_type = {
    input = bool
  }
  expr       = invert

  step "invert" {
    uses = "invert"
  }
}
