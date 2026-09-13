# Consumed by goreleaser, which supplies the already-built binary in the build
# context rather than compiling inside the image.
FROM alpine:3.20

# git is the tool's one runtime dependency. tini keeps signal handling sane
# when the container is stopped mid-analysis. The tool reads local repositories
# and never contacts a remote, so no CA bundle is requested.
RUN apk add --no-cache git tini

COPY commitography /usr/local/bin/commitography

# Analyzing a repository owned by another uid is the normal case for a mounted
# volume, and git refuses that by default. The setting goes in the system
# configuration so it also applies when the container runs with --user.
RUN git config --system --add safe.directory '*'

WORKDIR /repo

# The default command is the batch CLI. The local web server is opt-in, by
# overriding the command:
#
#   docker run --rm --publish 127.0.0.1:8080:8080 \
#     --mount type=bind,source="$PWD",target=/repos,readonly \
#     <image> serve --listen 0.0.0.0:8080 --allowed-root /repos
#
# There is deliberately no EXPOSE: it would let `docker run -P` publish the
# server on every host interface. There is no HEALTHCHECK either, because the
# server has no health endpoint beyond its documented API.
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/commitography"]
CMD ["/repo", "-o", "/repo/out"]
