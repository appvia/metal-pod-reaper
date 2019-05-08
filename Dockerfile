FROM golang:1.12
WORKDIR /go/src/github.com/appvia/metal-pod-reaper/
COPY . .
RUN make release

FROM alpine:3.8

RUN apk --update add libcap
COPY --from=0 /go/src/github.com/appvia/metal-pod-reaper/bin/mpodr_linux_amd64 /bin/mpodr
RUN chmod +x /bin/mpodr
RUN setcap cap_net_raw=+ep /bin/mpodr
RUN adduser -D -u 1000 mpodr
USER 1000

ENTRYPOINT ["/bin/mpodr"]
