trigger "cron" {
  schedule = "* * * * *"
}

step "prepare" {
  command = "printf 'prepare-ok'"
}

step "compile" {
  command = "printf 'compile-ok'"
}

step "test" {
  command = "printf 'test-ok'"
}

step "package" {
  expr = "package-ok"
}

job "bootstrap" {
  step_ref "prepare" {}
}

job "main" {
  step_ref "prepare" {}
  step_ref "compile" {
    depends_on = ["prepare"]
  }
  step_ref "test" {
    depends_on = ["compile"]
  }
  step_ref "package" {
    depends_on = ["compile", "test"]
  }
}

workflow "main" {
  trigger_ref "cron" {}
  job_ref "bootstrap" {}
  job_ref "main" {
    depends_on = ["bootstrap"]
  }
}
