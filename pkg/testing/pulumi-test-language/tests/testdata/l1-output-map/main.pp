output "empty" {
    value = {}
}

output "strings" {
    value = {
        "greeting": "Hello, world!",
        "farewell": "Goodbye, world!",
    }
}

output "numbers" {
    value = {
        "1": 1,
        "2": 2,
    }
}

// Test that keys don't get renamed
output "keys" {
    value = {
        "my.key": 1,
        "my-key": 2,
        "my_key": 3,
        "MY_KEY": 4,
        "mykey": 5,
        "MYKEY": 6,
    }
}
