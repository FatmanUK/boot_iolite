FROM alpine:latest

ARG VERSION
ARG STAMP
LABEL org.opencontainers.image.version=${VERSION}
LABEL org.opencontainers.image.created=${STAMP}

# Runs as root: it binds ports 67, 69 and 80 and needs host networking.
WORKDIR /opt/iolite
COPY iolite .
ENTRYPOINT ["/opt/iolite/iolite"]
