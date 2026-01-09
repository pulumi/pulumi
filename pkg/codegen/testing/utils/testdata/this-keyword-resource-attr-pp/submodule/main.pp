config "name" "string" { }

path = "fake/path"
forceDestroy = true
pgpKey = "fakekey"
passwordLength = 16
passwordResetRequired = true
iamAccessKeyStatus = "Active"

resource "this" "aws:iam/user:User" {
  options {
    range = 1
  }
  name                = name
  path                = path
  forceDestroy        = forceDestroy
}

resource "thisUserLoginProfile" "aws:iam/userLoginProfile:UserLoginProfile" {
  __logicalName = "this"
  options {
    range = 1
  }
  user                  = this[0].name
  pgpKey                = pgpKey
  passwordLength        = passwordLength
  passwordResetRequired = passwordResetRequired
}

resource "thisAccessKey" "aws:iam/accessKey:AccessKey" {
  __logicalName = "this"
  options {
    range = 1
  }
  user   = this[0].name
  pgpKey = pgpKey
  status = iamAccessKeyStatus
}

output "someOutput" {
  value = thisAccessKey[0].id
}
