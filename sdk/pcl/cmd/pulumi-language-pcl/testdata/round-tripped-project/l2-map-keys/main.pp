resource "prim" "primitive:index:Resource" {
    boolean = false
    float = 2.17
    integer = -12
    string = "Goodbye"
    numberArray = [0, 1]
    booleanMap = {
        "my key" = false,
        "my.key" = true,
        "my-key" = false,
        "my_key" = true,
        "MY_KEY" = false,
        "myKey" = true,
    }
}

resource "ref" "primitive-ref:index:Resource" {
    data = {
        boolean = false
        float = 2.17
        integer = -12
        string = "Goodbye"
        boolArray = [false, true]
        stringMap = {
            "my key" = "one",
            "my.key" = "two",
            "my-key" = "three",
            "my_key" = "four",
            "MY_KEY" = "five",
            "myKey" = "six",
        }
    }
}

resource "rref" "ref-ref:index:Resource" {
    data = {
        innerData = {
            boolean = false
            float = -2.17
            integer = 123
            string = "Goodbye"
            boolArray = []
            stringMap = {
                "my key" = "one",
                "my.key" = "two",
                "my-key" = "three",
                "my_key" = "four",
                "MY_KEY" = "five",
                "myKey" = "six",
            }
        }
        boolean = true
        float = 4.5
        integer = 1024
        string = "Hello"
        boolArray = []
        stringMap = {
            "my key" = "one",
            "my.key" = "two",
            "my-key" = "three",
            "my_key" = "four",
            "MY_KEY" = "five",
            "myKey" = "six",
        }
    }
}

resource "plains" "plain:index:Resource" {
    data = {
        innerData = {
            boolean = false
            float = 2.17
            integer = -12
            string = "Goodbye"
            boolArray = [false, true]
            stringMap = {
                "my key" = "one",
                "my.key" = "two",
                "my-key" = "three",
                "my_key" = "four",
                "MY_KEY" = "five",
                "myKey" = "six",
            }
        }
        boolean = true
        float = 4.5
        integer = 1024
        string = "Hello"
        boolArray = [true, false]
        stringMap = {
            "my key" = "one",
            "my.key" = "two",
            "my-key" = "three",
            "my_key" = "four",
            "MY_KEY" = "five",
            "myKey" = "six",
        }
    }
    nonPlainData = {
        innerData = {
            boolean = false
            float = 2.17
            integer = -12
            string = "Goodbye"
            boolArray = [false, true]
            stringMap = {
                "my key" = "one",
                "my.key" = "two",
                "my-key" = "three",
                "my_key" = "four",
                "MY_KEY" = "five",
                "myKey" = "six",
            }
        }
        boolean = true
        float = 4.5
        integer = 1024
        string = "Hello"
        boolArray = [true, false]
        stringMap = {
            "my key" = "one",
            "my.key" = "two",
            "my-key" = "three",
            "my_key" = "four",
            "MY_KEY" = "five",
            "myKey" = "six",
        }
    }
}
