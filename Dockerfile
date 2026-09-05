# Consumed by goreleaser, which supplies the already-built binary in the build
# context rather than compiling inside the image.
FROM alpine:3.20

# git is the tool's one runtime dependency. tini keeps signal handling sane
# when the container is stopped mid-analysis.
RUN apk add --no-cache git tini ca-certificates

COPY commitography /usr/local/bin/commitography

# Analyzing a repository owned by another uid is the normal case for a mounted
# volume, and git refuses that by default.
RUN git config --global --add safe.directory '*'

WORKDIR /repo

ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/commitography"]
CMD ["/repo", "-o", "/repo/out"]
