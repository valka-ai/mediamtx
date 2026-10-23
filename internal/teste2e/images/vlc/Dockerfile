FROM amd64/alpine:3.24@sha256:b50d677a0275e3d57ca0f5fa7dd9c9e4cbacb32da8d114703a438356f5eb1678

RUN apk add --no-cache \
    vlc

RUN adduser -D -H -s /bin/sh -u 9337 user

COPY start.sh /
RUN chmod +x /start.sh

RUN mkdir /out \
    && chown user:user /out

USER user
ENTRYPOINT [ "/start.sh" ]
