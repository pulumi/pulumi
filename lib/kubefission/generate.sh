#!/bin/sh
cidlc \
    kubefission idl/ \
    --recursive \
    --out-pack=pack/ \
    --out-rpc=rpc/

