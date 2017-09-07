{
    "targets": [
        {
            "target_name": "nativeruntime",
            "sources": [
                "closure.cc"
            ],
            "include_dirs": [
                "<!(node -e \"console.log(\`third_party/node/node-\${process.version}/deps/v8\`)\")"
            ]
        }
    ]
}
