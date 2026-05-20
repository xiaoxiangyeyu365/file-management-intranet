# Docker Deployment

## Quick Start

```bash
# Build and start the container
make docker-build
make docker-up

# View logs
make docker-logs

# Stop
make docker-down
```

## Manual Commands

```bash
# Build image
docker compose build

# Start in background
docker compose up -d

# View logs
docker compose logs -f

# Stop
docker compose down
```

## Configuration

The Docker deployment uses `configs/config.docker.yaml` for configuration. To customize:

1. Copy and modify the config:
   ```bash
   cp configs/config.docker.yaml configs/config.custom.yaml
   ```

2. Update `docker-compose.yml` to mount your config:
   ```yaml
   volumes:
     - ./configs/config.custom.yaml:/app/config.yaml:ro
   ```

## Data Persistence

All data (database, uploaded files, thumbnails) is stored in a Docker volume named `cloudbox-data`. To completely remove all data:

```bash
make docker-clean
```

## Production Considerations

1. **Change default credentials**: Update `admin.password` in the config file
2. **Use strong JWT secret**: Set a random string for `jwt.secret`
3. **Enable HTTPS**: Run behind a reverse proxy (nginx, Caddy, Traefik)
4. **Backup data**: Regularly backup the `cloudbox-data` volume

## Environment Variables

You can override config values using environment variables in `docker-compose.yml`:

```yaml
environment:
  - JWT_SECRET=your-secret-key
```

Supported environment variables:
- `JWT_SECRET` - JWT signing secret
- `TZ` - Timezone (default: Asia/Shanghai)
