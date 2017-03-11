#!/bin/bash
# A simple installation script for the ACMECorp security analyzer.

set -e                    # bail on errors

echo Compiling:
go build -o coco-analyzer-acmecorp_security
echo Done.

