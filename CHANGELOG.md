### 2023-06-15
- Add `v2/accounts/{account_id}/subscriptions` method to view wallet subscriptions

### 2023-06-10
- Add `raw_body` field to all messages struct (in transactions and traces). It contains hex-encoded BoC with message body. Optional field (message can have no body). 

### 2023-06-12

- Docker image now available on hub.docker.com

### 2023-06-30

- /v2/jettons

### 2023-07-04 

- /v2/pubkeys/{public_key}/wallets

### 2023-07-11

- /v2/blockchain/messages/{msg_id}/transaction

### 2023-07-17
- Add methods for get raw data, see /v2/liteserver/...
- /v2/rates supports changing percent the daily price