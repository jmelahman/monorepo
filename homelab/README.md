# Home Lab

## Create

If the certs don't yet exist, `nginx` will complain about the server with SSL enabled.
To avoid this, comment out all servers which have SSL enabled in `nginx/nginx.conf`.

Additionally, make the directories required for the ACME challenge,

```shell
mkdir -p certbot/www/.well-known/acme-challenge/
```

Restart nginx for the changes to take effect,

```shell
docker compose restart nginx
```

Finally, create the certs,

```shell
docker compose run --rm  certbot certonly --webroot --webroot-path /var/www/certbot/ -d lahman.dev,registry.lahman.dev,www.lahman.dev
```

_Note: If the first attempt fails, pass `--dry-run` to avoid being rate-limited._

## Renew

```shell
docker compose run --rm certbot renew
```

_Note: If the first attempt fails, pass `--dry-run` to avoid being rate-limited._

## Add Domains

Re-run the [Create](#create) command and select the Expand option.
Then, restart the nginx server with,

```shell
docker compose restart nginx
```
