# Overview

A simple Caddy plugin module to count the number of times a file has been accessed via Caddy.

## FrankenPHP Docker Build Example

In this example, I've added the module to a [FrankenPHP](https://frankenphp.dev/) Docker container.  FrankenPHP is based on Caddy and provides a single container to use Caddy but also run PHP based websites.  You may not need some of the PHP extensions and supporting libraries.

**Dockerfile:**
~~~
# Build FrankenPHP with the custom Caddy plugins
FROM dunglas/frankenphp:builder AS builder

COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy

RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    xcaddy build \
    --output /usr/local/bin/frankenphp \
    --with github.com/dunglas/frankenphp=./ \
    --with github.com/dunglas/frankenphp/caddy=./caddy/ \
    --with github.com/dunglas/caddy-cbrotli \
    --with github.com/SynapseCarbon/caddy-simple-stats

# Could also use an Alpine based image for a smaller image - alter code below as needed
FROM dunglas/frankenphp:1.12.3-php8-trixie

# Swap the binary
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp

# Install system dependencies for GD (jpeg, png, webp support) and SQLite.
# Uses apt on Debian rather than apk (Alpine).
RUN apt-get update && apt-get install -y --no-install-recommends \
        libpng-dev \
        libjpeg-dev \
        libwebp-dev \
        libfreetype-dev \
        libsqlite3-dev \
        libimage-exiftool-perl \
    && rm -rf /var/lib/apt/lists/*

# Configure and install PHP extensions as required
# install-php-extensions is bundled in the FrankenPHP image
RUN install-php-extensions \
       gd \
       redis

# Copy PHP runtime config (upload limits, timeouts, session settings) if needed
# COPY ./php.ini /usr/local/etc/php/conf.d/myphp.ini

# Set working directory — must match the root directive in Caddyfile
# and the bind mount paths in docker-compose.yml
WORKDIR /var/www/myapplication

# Run container as www-data, so setcap needed to bind to port 80
RUN setcap cap_net_bind_service=+ep /usr/local/bin/frankenphp
USER www-data

# FrankenPHP listens on 80 (HTTP) inside the container
EXPOSE 80

CMD ["frankenphp", "run", "--config", "/etc/caddy/Caddyfile"]
~~~

To check if the module has been built into your Caddy container, run:

~~~
docker exec dev-is-frankenphp frankenphp list-modules
~~~

In the non-standard modules section you should see the plugin:

```diff
admin.api.frankenphp
admin.api.mercure_health
frankenphp
http.encoders.br
http.handlers.mercure
http.handlers.mercure.bolt
http.handlers.mercure.local
http.handlers.php
+http.handlers.simple_stats

  Non-standard modules: 9
```


**Caddyfile**

In the global opions section:

```
{
    admin off
    frankenphp
    order simple_stats before method
    order php_server last
}
```

If you need to call the logging from different places in the file, you can use a snippet to have one block of code.  The Redis_ environment variables are pulled from a .env file supplied to the docker-compose file.

```
# Snippet for logging statistics to Caddy (when enabled)

(redis_logging) {
    simple_stats {
        redis_addr {$REDIS_CONTAINER_NAME}:{$REDIS_PORT}
        prefix {$REDIS_KEY_PREFIX}:
    }
}
```
Then if I wanted to count access to any static files on https://<mydomain>/uploads/*, the server block could look something like below.  If you will always log the number of requests then you obviously don't need to check Redis logging is enabled.

```
https://{DOMAIN} {

    root * /var/www/mywebsite

    # Check an environment variable that Redis logging is enabled
    @redis_enabled vars {env.REDIS_ENABLED} "true" "True"

    # Handle request to /uploads/*
    # Import and re-use the code snippet from earlier in the Caddyfile
    handle /uploads/* {
        handle @redis_enabled {
            import redis_logging
        }
        header Cache-Control "public, max-age=2592000, immutable"
        file_server
    }
```

## Testing

If everything is working and traffic is flowing to the URL been monitored for statistics, a command like below should show you the filenames and the access count:

```
docker exec <container-name> redis-cli HGETALL <redis-prefix>:urls
```
Result:
```
/uploads/1b28964481dc95f9.jpg
5
```
