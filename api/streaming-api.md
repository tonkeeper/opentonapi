# Streaming API

TonAPI V2 and opentonapi as well offer real-time updates about events in the blockchain.

## Authorization

All methods of the Streaming API are available without a private API key, 
but TonAPI limits a number of concurrent requests in this case. 
You should consider getting a personal API key at https://tonconsole.com/.
It's crucial for any production usage.

When you have an API key,  **Authorization** header has to be set to **Bearer <API-KEY>**:
```
Bearer eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9
```

## Server-Sent Events 

SSE methods response with `text/event-stream` Content-Type and communications happen in a text format.
Each API method sends two types of events: **heartbeat** and **message**.
The "heartbeat" event occurs every 5 seconds when nothing else happens 
and is a signal that everything is OK with an underlying TCP connection. 
The "message" event carries important information and its "data" always contains a JSON representation of a message.

[A golang example](https://github.com/tonkeeper/opentonapi/tree/master/examples/golang/sse) of working with SSE method.

### Real-time notifications about transactions

API method GET `https://tonapi.io/v2/sse/accounts/transactions?accounts=<comma-separated-list-of-accounts>` takes in
a comma-separated list of account IDs as "accounts" query parameter
and starts streaming transactions that belong to the given list of accounts.
A special value of "accounts" is **ALL**. TonAPI will stream all transactions in this case.

A response example:
```text
event: heartbeat

event: message
id: 1682407879253338019
data: {"account_id":"-1:5555555555555555555555555555555555555555555555555555555555555555","lt":37121532000003,"tx_hash":"076a457ace46c6bcea6ef0644d65a4b866d25a5fd52349f08a6ccfbf7cb99ddb"}
```

### Real-time notifications about pending messages (Mempool).
API method GET 'https://tonapi.io/v2/sse/mempool' immediately starts streaming BOCs of pending inbound messages:

```text
event: heartbeat

event: message
id: 1682342934235516717
data: {"boc":"te6ccgEBBAEAtwABRYgBvVXMoxQj+kmDtTinWnFdumvpTNo33p48YQKOWyTtUkAMAQGcMZ6id5dkoDZImQ4UC5SqZSN04h/xNpKaEsESJQivKW01aMcWW4qeUUjKm/iZ2nszwBj3uFVcsIr9xFomQvY3DCmpoxdkQjldAAAAcAADAgFkQgAoPvU+sDeRbPQrPGn3bxzd8JnUNGlQcfA/qoFluFxSiRE4gAAAAAAAAAAAAAAAAAEDABIAAAAAaGVsbG8="}
```

## Websocket

TonAPI supports a JSON-RPC protocol over a websocket connection. It is available at `wss://tonapi.io/v2/websocket`.   
Supported methods are: 
* **subscribe_account**
* **subscribe_mempool**

[A golang example](https://github.com/tonkeeper/opentonapi/tree/master/examples/golang/websocket) of working with websocket.

### "subscribe_account" method
`subscribe_account` takes in a list of account IDs as "params" argument 
and stars streaming transactions that belong to the given list of accounts.
A request example: 
```json
{
  "id":1,
  "jsonrpc":"2.0",
  "method":"subscribe_account",
  "params":[
    "-1:5555555555555555555555555555555555555555555555555555555555555555",
    "-1:3333333333333333333333333333333333333333333333333333333333333333"
  ]
}
```
A response:
```json
{
  "id":1,
  "jsonrpc":"2.0",
  "method":"subscribe_account",
  "result":"success! 2 new subscriptions created"
}
```

When a transaction is committed to the blockchain, TonAPI will send a short summary about it:
```json
 {
  "jsonrpc":"2.0",
  "method":"account_transaction",
  "params":{
    "account_id":"-1:5555555555555555555555555555555555555555555555555555555555555555",
    "lt":37121758000003,
    "tx_hash":"586e176bdead2a37d9e372c3725e27c4eab90f5b213c6099c6aadeafc8e4fbc9"
  }
}
```

It is possible to subscribe up to 1000 accounts per a websocket connection.

###  "subscribe_mempool" method

`subscribe_mempool` subscribes you to notifications about pending inbound messages.  
A request example: 

```json
{
  "id":1,
  "jsonrpc":"2.0",
  "method":"subscribe_mempool"
}
```
A response:
```json
{
  "id":1,
  "jsonrpc":"2.0",
  "method":"subscribe_mempool",
  "result":"success! you have subscribed to mempool"
}
```
An example of a new-pending-message event:
```json
{
  "jsonrpc":"2.0",
  "method":"mempool_message",
  "params":{
    "boc":"te6ccgEBBAEAtwABRYgBvVXMoxQj+kmDtTinWnFdumvpTNo33p48YQKOWyTtUkAMAQGcMZ6id5dkoDZImQ4UC5SqZSN04h/xNpKaEsESJQivKW01aMcWW4qeUUjKm/iZ2nszwBj3uFVcsIr9xFomQvY3DCmpoxdkQjldAAAAcAADAgFkQgAoPvU+sDeRbPQrPGn3bxzd8JnUNGlQcfA/qoFluFxSiRE4gAAAAAAAAAAAAAAAAAEDABIAAAAAaGVsbG8="
  }
}
```
