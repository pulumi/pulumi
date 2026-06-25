# Program image for the OCI smoke test. The program is a statically-linked Go
# binary cross-compiled on the host (see run.sh), so the image is just the
# binary on a minimal base — no SDK install, no language toolchain. The base
# image's only job is to provide the program's runtime shape; here that is
# nothing more than libc-free alpine.
FROM alpine:3
COPY program-linux /program
ENTRYPOINT ["/program"]
