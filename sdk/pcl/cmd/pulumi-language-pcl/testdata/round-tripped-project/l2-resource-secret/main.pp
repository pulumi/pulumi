resource "res" "secret:index:Resource" {
    private = "closed"
    public = "open"
    privateData = {
        private = "closed"
        public = "open"
    }
    publicData = {
        private = "closed"
        public = "open"
    }
    privateArray = ["closed"]
    privateMap = {
        "key" = "closed"
    }
    privateDataArray = [{
        private = "closed"
        public = "open"
    }]
    privateDataMap = {
        "key" = {
            private = "closed"
            public = "open"
        }
    }
}
