# jsonrpc-proxy ![Build and Release](https://github.com/Revolution1/jsonrpc-proxy/workflows/Build%20and%20Release/badge.svg)

# Quick Start

### Install

```shell
go install github.com/revolution1/jsonrpc-proxy

#or

curl -L https://github.com/Revolution1/jsonrpc-proxy/releases/latest/download/jsonrpc-proxy-Darwin-amd64 -o jsonrpc-proxy
chmod +x jsonrpc-proxy
```

### Edit Config and Run

Download config file from https://github.com/Revolution1/jsonrpc-proxy/raw/main/proxy.yaml

Run

```shell
jsonrpc-proxy -c proxy.yaml
```

### Test

```shell
$ curl http://loclahost:8080 -X POST -d '{"id":1,"jsonrpc":"2.0","method":"GetBlockchainInfo","params":[]}'
```

## Request Processing & Caching Policy

```text
path not match:
\_ return 404 Not Found
path match:
\_ invalid json: return -32700 Parse Error
\_ valid json
   \_ one request & jsonrpc invalid: return -32600 Invalid Request
   \_ valid jsonrpc
      \_ one request:
         \_ cached: return cached response
         \_ not cached: forward to upstream
            \_ net|http|jsonrpc error: cache error for 'ErrFor' duration
            \_ success: cache for 'for' duration and return
      \_ batch request:
         \_ all invalid: return errors
         \_ all cached: return cached responses
         \_ not all cached: forward whole batch to upstream
            \_ net|http error: cache error for 'ErrFor' duration
            \_ jsonrpc error: cache errored request for it's 'ErrFor' duration
            \_ success: cache for it's 'for' duration and return
```

# TODO

- [x] batch request
- [ ] k8s service discovery
- [ ] cache notfound error
- [ ] method statistics
- [ ] account based rate limiting
- [ ] epoch based retry & loadbalancing
- [ ] modularize
- [ ] easyjson & msgp


# ref

https://www.jsonrpc.org/historical/json-rpc-over-http.html

