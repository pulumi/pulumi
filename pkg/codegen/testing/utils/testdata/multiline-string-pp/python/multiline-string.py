import pulumi
import pulumi_random as random

foo = random.RandomShuffle("foo", inputs=[
    """just one
newline""",
    """foo
bar
baz
qux
quux
qux""",
    """{
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
""",
])
