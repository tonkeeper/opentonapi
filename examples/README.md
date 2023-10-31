

## Examples

This directory contains examples of how to use TonAPI SDK to work with the TON Blockchain.

## HTTP API

We believe that the native TON's RPC is very low-level. 
And it is not suitable for building applications on top of it.  

TonAPI/Opentonapi aims at speeding up development of TON-based applications and 
provides an API centered around high-level concepts like Jettons, NFTs and so on,
keeping a way to access low-level details.

[TonAPI SDK example](golang/tonapi-sdk/main.go) shows how to work with TonAPI/Opentonapi HTTP API. 

## Streaming API

Usually, an application needs to monitor the blockchain for specific events and act accordingly.    
TonAPI/Opentonapi offers two ways to do it: SSE and Websocket.   

The advantage of Websocket is that Websocket can be reconfigured dynamically to subscribe/unsubscribe to/from specific events. 
Where SSE has to reconnect to TonAPI/Opentonapi to change the list of events it is subscribed to. 

Take a look at [SSE example](golang/sse/main.go) and [Websocket example](golang/websocket/main.go) to see how to work with TonAPI/Opentonapi Streaming API.


