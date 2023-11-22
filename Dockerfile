FROM cgr.dev/chainguard/static:latest
COPY sa-key-rotator /sa-key-rotator
ENTRYPOINT ["/sa-key-rotator"]