FROM ubuntu:22.04

# Set the timezone and install CA certificates
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates tzdata && rm -rf /var/lib/apt/lists/*

COPY lantern-server-manager /app/server
COPY --from=getlantern/sing-box-extensions /usr/local/bin/lantern-box /usr/local/bin/lantern-box

# Set the entrypoint command
ENTRYPOINT ["/app/server", "serve"]
