FROM amd64/alpine:3.24@sha256:b50d677a0275e3d57ca0f5fa7dd9c9e4cbacb32da8d114703a438356f5eb1678

RUN apk add --no-cache \
    ffmpeg

COPY *.mkv /

COPY start.sh /
RUN chmod +x /start.sh

ENTRYPOINT [ "/start.sh" ]
