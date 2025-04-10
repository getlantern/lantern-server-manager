FROM alpine:edge

# Set the timezone and install CA certificates
RUN apk --no-cache add ca-certificates tzdata

COPY lantern-server-manager /app/server
COPY --from=ghcr.io/sagernet/sing-box /usr/local/bin/sing-box /usr/local/bin/sing-box

# Set the entrypoint command
ENTRYPOINT ["/app/server", "serve"]