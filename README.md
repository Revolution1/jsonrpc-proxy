# jsonrpc-proxy ![Build and Release](https://github.com/Revolution1/jsonrpc-proxy/workflows/Build%20and%20Release/badge.svg)

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
- [ ] cache notfound error
- [ ] method statistics
- [ ] account based rate limiting
- [ ] epoch based retry & loadbalancing
- [ ] modularize
- [ ] easyjson & msgp
- [ ] consider https://dgraph.io/blog/post/introducing-ristretto-high-perf-go-cache/
- [ ] replace logrus with zerolog

# ref

https://www.jsonrpc.org/historical/json-rpc-over-http.html

