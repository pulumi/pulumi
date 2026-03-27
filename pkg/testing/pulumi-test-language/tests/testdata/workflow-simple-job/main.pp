workflow "main" {
  job "build" {
    step "constant" {
      output_type = "string"
      expr        = "\"done\""
    }
  }
}
