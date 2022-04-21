FROM alpine:latest
# TODO(jamison): error: failed to solve: executor failed running [/bin/sh -c apk --no-cache add ca-certificates]: exit code: 1
#RUN apk --no-cache add ca-certificates
COPY lfscache /bin/lfscache
ENTRYPOINT ["/bin/lfscache"]
