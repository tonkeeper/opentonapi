
# Description

Opentonapi simplifies development of TON-based applications and 
provides an API centered around high-level concepts like Jettons, NFTs and so on keeping a way to access low-level details.

Opentonapi is an open source version of [TonAPI](http://tonapi.io) and is 100% compatible with it.

The main difference is that TonAPI maintains an internal index of the whole TON blockchain and 
provides information about any entity in the blockchain including Jettons, NFTs, accounts, transactions, traces, etc.

# TonAPI SDK and API documentation

Development of TON-based applications is much easier with TonAPI SDK:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tonkeeper/opentonapi/tonapi"
)

func main() {
	client, err := tonapi.New()
	if err != nil {
		log.Fatal(err)
	}
	account, err := client.GetAccount(context.Background(), tonapi.GetAccountParams{AccountID: "EQBszTJahYw3lpP64ryqscKQaDGk4QpsO7RO6LYVvKHSINS0"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Account: %v\n", account.Balance)
}
```

Take a look at more examples at [TonAPI SDK examples](examples/README.md).


[Openapi.yaml](api/openapi.yml) describes the API of both Opentonapi and TonAPI.

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