### 2023-06-15
- Add `v2/accounts/{account_id}/subscriptions` method to view wallet subscriptions

### 2023-06-10
- Add `raw_body` field to all messages struct (in transactions and traces). It contains hex-encoded BoC with message body. Optional field (message can have no body). 

### 2023-06-12
- Docker image now available on hub.docker.com

### 2023-06-30
- `/v2/jettons`

### 2023-07-04 
- `/v2/pubkeys/{public_key}/wallets`

### 2023-07-11
- `/v2/blockchain/messages/{msg_id}/transaction`

### 2023-08-09
- Get the account event by event_id `/v2/accounts/{account_id}/events/{event_id}`
- Get detail DNS info, support .ton and t.me records `/v2/dns/{dns}`
- Get the account traces `/v2/accounts/{account_id}/traces`
- Support send batch from boc `/v2/blockchain/message`

### 2023-10-01
- Tonapi sdk

### 2023-10-21
- `/v2/blockchain/masterchain/{masterchain_seqno}/config` method which returns a blockchain config for a specific block.
- `/v2/blockchain/masterchain/{masterchain_seqno}/config/raw` method which returns a raw blockchain config for a specific block.

### 2023-10-22
- `/v2/blockchain/validators` method which returns the current validators and theirs stakes.
 
### 2023-10-23
- `/v2/blockchain/blocks/{block_id}` returns a value flow from a block.
