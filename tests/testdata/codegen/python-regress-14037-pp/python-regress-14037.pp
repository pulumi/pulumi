data = ["bob", "john", "carl"]

resource "user" "random:index/randomPassword:RandomPassword" {
  options { range = data }
  length = 16
}
resource "dbUsers" "aws:secretsmanager/secretVersion:SecretVersion" {
  options { range = data }
  secretId = "mySecret"
  secretString = toJSON({
    username = range.value
    password = user[range.value].result
  })
}
