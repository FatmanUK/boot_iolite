# Stage 1: Create a minimal runtime image
FROM alpine:latest

# Import variables
ARG BUILD_TIMESTAMP
ARG VERSION
ARG APP_BINARY
ARG APP_PATH='/opt'

# Bridge ARG to ENV
ENV BINARY=${APP_BINARY}
ENV BPATH=${APP_PATH}

# Labels for date, timestamp, and version
LABEL org.opencontainers.image.created=${BUILD_TIMESTAMP}
LABEL org.opencontainers.image.version=${VERSION}

# Set opt workdir
WORKDIR ${APP_PATH}/
COPY ${APP_BINARY} .

USER 1000
ENTRYPOINT ["/bin/sh", "-c", "${BPATH}/${BINARY} \"$@\"", "--"]
