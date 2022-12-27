# Status

A status page for the services running in the lab.
An example is hosted at https://status.lahman.dev.

<img src="demo.png" alt="drawing" width="500"/>

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