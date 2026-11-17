resource "source" "bytesource:index:Resource" {
    base64 = "AGhlbGxvIID+/yB3b3JsZPAo"
}

resource "sink" "bytesink:index:Resource" {
    bytes = source.bytes
    expectBase64 = source.base64
}
