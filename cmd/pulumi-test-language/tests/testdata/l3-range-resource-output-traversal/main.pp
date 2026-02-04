resource "subscription" "dns:index:Subscription" {
  domains = ["example.com", "test.com"]
}

resource "record" "dns:index:Record" {
  options {
    range = subscription.challenges
  }
  name = range.value.recordName
}
