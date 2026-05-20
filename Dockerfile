# Minimal runtime with static busybox
FROM busybox:1.36-glibc

WORKDIR /app

# Copy binary, config and static files
COPY cloudbox /app/cloudbox
COPY configs/config.docker.yaml /app/config.yaml
COPY static/ /app/static/

# Create data directories
RUN mkdir -p /app/data/files /app/data/temp /app/data/thumbnails /app/data/logs

EXPOSE 8080/tcp

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/auth/login > /dev/null || exit 1

CMD ["/app/cloudbox"]