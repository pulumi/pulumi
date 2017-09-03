{
    "targets": [
        {
            "target_name": "nativeruntime",
            "sources": [
                "closure.cc"
            ],
            "include_dirs": [
                "<!(node -e \"console.log(\`third_party/node/\${process.version.substring(1)}/deps/v8\`)\")"
            ]
        }
    ]
}
