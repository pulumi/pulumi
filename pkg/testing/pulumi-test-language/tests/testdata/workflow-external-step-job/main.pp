package external {
  version = "1.0.0"
}

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
    uses = "external:invert"
  }
}
