# Status

A status page for the services running in the lab.

## Run

```shell
bazel run //lab/status:app
```

## Build

```shell
docker build -t status .
```

## Run Docker

```shell
docker run -p 5000:5000 -v /home/jamison/.ssh/:/root/.ssh/:ro status
```

## Deploy

```shell
cd ../
docker-compose up -d status
```