step "constant" {
  output_type = "string"
  expr        = "\"done\""
}

workflow "main" {
  job "build" {
    step "constant" {
      uses = "constant"
    }
  }
}
