FROM alpine:3.8

RUN apk upgrade --no-cache
COPY bin/mpodr_linux_amd64 /bin/mpodr

RUN chmod +x /bin/mpodr
RUN adduser -D -u 1000 mpodr
USER 1000

ENTRYPOINT ["/bin/mpodr"]
