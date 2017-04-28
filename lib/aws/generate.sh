#!/bin/sh
cidlc \
    aws idl/ \
    --recursive \
    --out-pack=pack/ \
    --out-rpc=rpc/

