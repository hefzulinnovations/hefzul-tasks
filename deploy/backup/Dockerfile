FROM alpine:3.21

RUN apk add --no-cache bash postgresql17-client rclone tar gzip tzdata coreutils

COPY backup.sh restore.sh scheduler.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/backup.sh /usr/local/bin/restore.sh /usr/local/bin/scheduler.sh

CMD ["/usr/local/bin/scheduler.sh"]
