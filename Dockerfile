FROM ubuntu:22.04

ARG TARGETPLATFORM

COPY cloud/lantern-packages.asc /etc/apt/keyrings/lantern-packages.asc

# Install CA certificates, timezone data, and lantern-box (packaged as sing-box-extensions)
# from the Lantern apt repository, which publishes it for every architecture we build.
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates tzdata && \
    echo "deb [signed-by=/etc/apt/keyrings/lantern-packages.asc] https://apt.fury.io/getlantern/ /" > /etc/apt/sources.list.d/fury.list && \
    apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y sing-box-extensions && \
    rm -rf /var/lib/apt/lists/*

COPY $TARGETPLATFORM/lantern-server-manager /app/server

# Set the entrypoint command
ENTRYPOINT ["/app/server", "serve"]
