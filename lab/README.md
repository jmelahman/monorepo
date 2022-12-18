# Home Lab

## Create

```shell
docker-compose run --rm  certbot certonly --webroot --webroot-path /var/www/certbot/ -d lahman.dev,build.lahman.dev,www.lahman.dev
```

## Renew

```shell
docker-compose run --rm certbot renew --dry-run
```

```shell
docker-compose run --rm certbot renew
```

## Add Domains

Re-run the [Create](#create) command and select the Expand option.
