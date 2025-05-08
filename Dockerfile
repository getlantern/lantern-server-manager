FROM alpine:edge

# Set the timezone and install CA certificates
RUN apk --no-cache add ca-certificates tzdata

COPY lantern-server-manager /app/server
COPY --from=getlantern/sing-box-extensions /sing-box-extensions /usr/local/bin/sing-box-extensions

# Set the entrypoint command
ENTRYPOINT ["/app/server", "serve"]