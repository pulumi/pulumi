package simple-step-workflow {
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
    uses = "simple-step-workflow:invert"
  }
}
