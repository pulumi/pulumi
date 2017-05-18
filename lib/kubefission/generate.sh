#!/bin/sh
lumidl \
    kubefission idl/ \
    --recursive \
    --out-pack=pack/ \
    --out-rpc=rpc/

