config input string {
    description = "A simple input"
}

config cidrBlocks "map(string)" {
    description = "The main CIDR blocks for the VPC\nIt is a map of strings"
}

config "githubApp" "object({id=string, keyBase64=string,webhookSecret=string})" {
  description = "GitHub app parameters, see your github app. Ensure the key is the base64-encoded `.pem` file (the output of `base64 app.private-key.pem`, not the content of `private-key.pem`)."
  nullable = true
}

config "servers" "list(object({name=string}))" {
  description = "A list of servers"
  nullable = true
}

config "deploymentZones" "map(object({ zone = string }))" {
  description = "A map between for zones"
  nullable = true
}

config ipAddress "list(int)" { }

resource password "random:index/randomPassword:RandomPassword" {
  length = 16
  special = true
  overrideSpecial = input
}

resource githubPassword "random:index/randomPassword:RandomPassword" {
  length = 16
  special = true
  overrideSpecial = githubApp.webhookSecret
}

# Example of iterating a list of objects
resource serverPasswords "random:index/randomPassword:RandomPassword" {
   options { range = length(servers) }
   length = 16
   special = true
   overrideSpecial = servers[range.value].name
}

# Example of iterating a map of objects
resource zonePasswords "random:index/randomPassword:RandomPassword" {
   options { range = deploymentZones }
   length = 16
   special = true
   overrideSpecial = range.value.zone
}

component simpleComponent "../simpleComponent" {}

output result {
    value = password.result
}