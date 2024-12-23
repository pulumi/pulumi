# use supabase because there is no pulumi package for it.
terraform {
  required_providers {
    supabase = {
      source  = "supabase/supabase"
      version = "~> 1.0"
    }
  }
}

resource "supabase_project" "my_project" {
  name   = "my-supabase-database"
  region = "us-east-1"
  database_password = "password"
  organization_id = "organization_id"
}
