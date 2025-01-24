terraform {
  required_providers {
    b2 = {
      source = "Backblaze/b2"
      version = "0.10.0"
    }
    supabase = {
      source  = "supabase/supabase"
      version = "~> 1.0"
    }
  }
}

provider b2 {}
provider supabase {}
