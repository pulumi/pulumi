#!/bin/bash
# A simple installation script for a basic infosec scanner.

set -e                    # bail on errors

echo Compiling:
go build -o coco-analyzer-contoso_infosec
echo Done.

