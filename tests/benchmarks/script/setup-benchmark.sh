#!/usr/bin/env bash

set -euo pipefail

# Purge hardware cache (MacOS specific)
sync && sudo purge
