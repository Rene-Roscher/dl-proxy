# Download Proxy

A smart caching proxy for large file downloads with S3/R2 storage.

## Features

- **Smart Caching**: Only caches files after N requests in a time window (default: 10 requests in 1 hour)
- **Streaming**: Files stream directly to clients while uploading to S3/R2 (no memory buffering)
- **Auto-Cleanup**: Deletes cached files not requested for 30+ days
- **SQLite by Default**: Simple setup with no external database needed
- **S3/R2 Compatible**: Works with AWS S3 and Cloudflare R2

## Quick Start (SQLite)

1. **Copy environment file:**
```bash
cp .env.example .env
```

2. **Edit .env with your R2/S3 credentials:**
```bash
AWS_ACCESS_KEY_ID=your_key
AWS_SECRET_ACCESS_KEY=your_secret
S3_BUCKET=your-bucket
S3_ENDPOINT=https://your-account-id.r2.cloudflarestorage.com
```

3. **Start with Docker:**
```bash
docker-compose up -d
```

4. **Test it:**
```bash
curl "http://localhost:1330/proxy?url=https://example.com/file.zip"
```

That's it! The proxy runs on port 1330 with SQLite database at `./data/proxy.db`.

## How It Works

1. **First few requests** (< threshold): File is proxied directly to client, not cached
2. **Threshold reached** (≥10 requests in 1 hour): File is streamed to client AND uploaded to R2
3. **Subsequent requests**: Client redirected to R2 presigned URL (fast!)
4. **Cleanup**: Files unused for 30+ days are automatically deleted

## Configuration

Key settings in `.env`:

```bash
# Smart Caching
CACHE_THRESHOLD=10              # Requests needed before caching
CACHE_WINDOW=1h                 # Time window for counting requests
CACHE_RETENTION_DAYS=30         # Delete after X days unused

# Database (SQLite default)
DB_PATH=./data/proxy.db
```

## Using MariaDB Instead

For production with MariaDB + phpMyAdmin:

```bash
docker-compose -f docker-compose.mariadb.yml up -d
```

Update `.env`:
```bash
DB_TYPE=mysql
DB_HOST=db
DB_USER=proxy_user
DB_PASSWORD=your_password
```

- **Proxy**: http://localhost:1330
- **phpMyAdmin**: http://localhost:1331 (auto-login)

## API

**Proxy a download:**
```
GET /proxy?url=<encoded_url>
```

**Health check:**
```
GET /health
```

## Development

**Run tests:**
```bash
go test ./...
```

**Build binary:**
```bash
go build -o proxy
```

**Run locally:**
```bash
cp .env.example .env
# Edit .env with your credentials
go run .
```

## How URLs Are Cached

URLs are normalized before caching:
- Protocol removed (`https://` → ` `)
- Lowercase host/path
- Default ports removed (`:443`, `:80`)
- Query params sorted (preserves values case)
- MD5 hash as cache key

Example:
```
https://EXAMPLE.com:443/File.zip?token=ABC&v=2
→ example.com/file.zip?token=ABC&v=2
→ MD5: a1b2c3d4...
```

## Monitoring

Check logs:
```bash
docker-compose logs -f proxy
```

Database (SQLite):
```bash
sqlite3 ./data/proxy.db "SELECT * FROM files WHERE cached=1;"
```

## Architecture

```
Client Request
     ↓
[URL Normalization] → MD5 Cache Key
     ↓
[Database Check]
     ↓
├─ Cached? → 302 Redirect to R2
├─ Below Threshold? → Proxy without caching
└─ Threshold Reached? → Stream to client + Upload to R2
```

## License

MIT
