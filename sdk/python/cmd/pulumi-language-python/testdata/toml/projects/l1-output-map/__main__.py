import pulumi

pulumi.export("empty", {})
pulumi.export("strings", {
    "greeting": "Hello, world!",
    "farewell": "Goodbye, world!",
})
pulumi.export("numbers", {
    "1": 1,
    "2": 2,
})
pulumi.export("keys", {
    "my.key": 1,
    "my-key": 2,
    "my_key": 3,
    "MY_KEY": 4,
    "mykey": 5,
    "MYKEY": 6,
})
