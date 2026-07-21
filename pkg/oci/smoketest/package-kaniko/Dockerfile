# Trivial hermetic image for the kaniko->CRI build+publish smoke test. FROM scratch
# needs no base pull; the COPY exercises source-reaching (marker.txt is read from the
# builder's inherited source mount). The image is published, not run.
FROM scratch
COPY marker.txt /marker.txt
