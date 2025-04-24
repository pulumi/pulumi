resource foo "random:index/randomShuffle:RandomShuffle" {
  inputs = [
    "just one\nnewline",
    "foo\nbar\nbaz\nqux\nquux\nqux",
    <<-EOT
      {
          "a": 1,
          "b": 2,
          "c": [
            "foo",
            "bar",
            "baz",
            "qux",
            "quux"
          ]
      }
    EOT
  ]
}
