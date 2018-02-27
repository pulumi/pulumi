{
    "targets": [
        {
            "target_name": "nativeruntime-v0.11.0",
            "sources": [
                "closure.cc"
            ],
            "conditions": [
                [ 'OS=="win"',
                    {
                        "include_dirs": [
                            "<!(node -e \"console.log(`third_party/node/node-${process.version}/deps/v8`)\")"
                        ]
                    },
                    {
                        "include_dirs": [
                            "<!(node -e \"console.log(\`third_party/node/node-\${process.version}/deps/v8\`)\")"
                        ]
                    }
                ]
            ]
        }
    ]
}
