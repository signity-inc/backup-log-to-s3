FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY backup-log-to-s3 .

# Create non-root user
RUN adduser -D -s /bin/sh backup

USER backup

ENTRYPOINT ["./backup-log-to-s3"]