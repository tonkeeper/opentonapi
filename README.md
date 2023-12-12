
# Description

Opentonapi simplifies development of TON-based applications and 
provides an API centered around high-level concepts like Jettons, NFTs and so on keeping a way to access low-level details.

Opentonapi is an open source version of [TonAPI](http://tonapi.io) and is 100% compatible with it.

The main difference is that TonAPI maintains an internal index of the whole TON blockchain and 
provides information about any entity in the blockchain including Jettons, NFTs, accounts, transactions, traces, etc.

# TonAPI SDK and API documentation

[Openapi.yaml](api/openapi.yml) describes the API of both Opentonapi and TonAPI.

For more examples and golang SDK take a look at [TonAPI SDK](https://github.com/tonkeeper/tonapi-go).

# How to run opentonapi

It is possible to use the environment variables listed below to configure opentonapi:

| Env variable | Default value | Comment                                                                                                                                                                                        |
|--------------|---------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| PORT         | 8081          | A port number used to accept incoming http connections                                                                                                                                         | 
| LOG_LEVEL    | INFO          | Log level                                                                                                                                                                                      | 
| LITE_SERVERS | -             | A comma-separated list of TON lite servers to work with. Each server has the following format: **ip:port:public-key**. <br/>Ex: "127.0.0.1:14395:6PGkPQSbyFp12esf1NqmDOaLoFA8i9+Mp5+cAx5wtTU=" | 
| METRICS_PORT | 9010          | A port number used to expose `/metrics` endpoint with prometheus metrics                                                                                                                       | 
| ACCOUNTS     | -             | A comma-separated list of accounts to watch for                                                                                                                                                | 


Advanced features like traces, NFTs, Jettons, etc require you to configure a set of accounts to watch for: 

```shell
ACCOUNTS="comma-separated-list-of-raw-account-addresses" make run 
```

## Docker

docker run -d -p8081:8081 tonkeeper/opentonapi 