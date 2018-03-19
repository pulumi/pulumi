#!/bin/bash

mutationIndex=$(($1 + 1))
cat $(find . -name "*.json.*" | sort | sed -n "${mutationIndex}p")
