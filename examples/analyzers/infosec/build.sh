#!/bin/bash
# A simple installation script for a basic infosec scanner.

set -e                    # bail on errors

echo Compiling:
go build -i -o lumi-analyzer-contoso_infosec
echo Done.

