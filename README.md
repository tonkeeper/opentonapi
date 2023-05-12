
# Description

Opentonapi provides a REST-ish API to work with the TON Blockchain.

# How to run 

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
