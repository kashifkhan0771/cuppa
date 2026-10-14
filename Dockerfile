# Used by GoReleaser, which builds the binaries first and puts each
# one under <os>/<arch>/ in the build context. Build it with
# "make release-snapshot", not "docker build".

# tiny base with no shell, running as a non-root user
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/cuppa /usr/bin/cuppa

# mount your cuppa.yaml here; it is found as ./cuppa.yaml
WORKDIR /work

ENTRYPOINT ["/usr/bin/cuppa"]
CMD ["--help"]
