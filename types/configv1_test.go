package types

import (
	assertion "github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	assert := assertion.New(t)
	conf := &Config{}
	err := yaml.Unmarshal([]byte(v1Example), conf)
	assert.NoError(err)
}

const v1Example = `
version: 1.0

listeners:
  - id: http
    address: 0.0.0.0:4201
  - id: tcp
    address: 0.0.0.0:4203
  - id: manage
    address: 0.0.0.0:8088

upstreams:
  - name: seedpub
    k8s_service:
      port: 4201
      namespace: default
      name: xxx
  - name: mainnet
    endpoints:
      - https://api.zilliqa.com


servers:
  - name: zilliqa-rpc
    type: http
    listeners:
    - http
    plugins:
      - id: cors
    routers:
      - path: /
        plugins:
        - id: logging
          format: text # text | json
          verbose: 1 # 1 2 3
          stream: stdout
        - id: zilliqa-api

      - path: /ws
        plugins:
        - id: websocket
        - id: websocket-to-simple-jsonrpc
        - id: logging
          format: text # text | json
          verbose: 1 # 1 2 3
          stream: stdout
        - id: zilliqa-api

      # https://pkg.go.dev/github.com/fasthttp/router
      - path: /manage/{path:*}
        plugins:
        - id: basic-auth
          username: admin
          password: admin
        - id: manage

  - name: zilliqa-tcp
    type: tcp
    listeners:
    - tcp
    KeepAlive: true
    Delimiter: '\n'
    plugins:
    - id: tcp-to-simple-jsonrpc
    - id: zilliqa-api


# user-defined plugin
processors:
  - id: zilliqa-api
    description: proxy jsonrpc request to zilliqa server and cache the responses
    context: jsonrpc
    plugins:
    - id: jsonrpc
      supportBatch: true
      keepAlive: true
      forward:
        requestTimeout: 10s
        supportBatch: true
        keepAlive: true
        upstreams:
          - seedpub
        defaultResponse:
          code: 503
          content: "Service Unavaliable"
          headers: null
    - id: cache
      maxSize: 256Mb
      errFor: 1s
      groups:
      - methods:
        # method that has no
        - GetBlockchainInfo
        - GetCurrentDSEpoch
        - GetCurrentMiniEpoch
        - GetDSBlockRate
        - GetLatestDsBlock
        - GetLatestTxBlock
        - GetNumDSBlocks
        - GetNumTransactions
        - GetNumTxBlocks
        - GetPrevDifficulty
        - GetPrevDSDifficulty
        - GetTotalCoinSupply
        - GetTransactionRate
        - GetTxBlockRate
        - GetMinimumGasPrice
        - GetNumTxnsDSEpoch
        - GetNumTxnsTxEpoch
        - GetPendingTxn
        - GetPendingTxns
        - GetRecentTransactions
        # method that has param
        - DSBlockListing
        - TxBlockListing
        - GetDsBlock
        - GetTxBlock
        - GetTransaction
        - GetTransactionsForTxBlock
        - GetTxnBodiesForTxBlock
        - GetSmartContractInit
        - GetSmartContracts
        - GetSmartContractState
        - GetSmartContractSubState
        - GetBalance
        for: 5s
        errFor: 1s
      # long term
      - methods:
        - GetContractAddressFromTransactionID
        - GetSmartContractCode
        for: 1h
        errFor: 1s
      # Permanent
      - methods:
        - GetNetworkId
        for: 1h
        errFor: 1s
`
