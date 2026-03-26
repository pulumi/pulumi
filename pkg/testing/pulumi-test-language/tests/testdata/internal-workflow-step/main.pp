step "echo" {
  input_type  = "internal:workflow:StepInput"
  output_type = "internal:workflow:StepOutput"
  command     = "printf step-output"
}

workflow "main" {
  job "build" {
    step "echo" {
      uses = "echo"
    }
  }
}

