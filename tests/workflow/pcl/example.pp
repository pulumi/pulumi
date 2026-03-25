workflow "main" {
  trigger "cron" {
    schedule = "* * * * *"
  }

  job "main" {
    step "run" {
      command = "printf 'pcl-task-ok'"
    }
  }
}
