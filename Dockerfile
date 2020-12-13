FROM golang AS builder
WORKDIR /jsonrpc-proxy
COPY . /jsonrpc-proxy
RUN ls -l && make local

FROM ubuntu:20.04
WORKDIR /root
COPY --from=builder /jsonrpc-proxy/dist/jsonrpc-proxy /usr/bin/
COPY proxy.yaml proxy.yaml
ENTRYPOINT ["jsonrpc-proxy"]
