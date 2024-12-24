# Setup and Run

This section provides step-by-step instructions for setting up and running the **OpenTonAPI** project locally. You can choose one of the following methods:

1. **Running via Docker**
2. **Running from Source Code with Go**

## Running via Docker

To run the project using Docker, follow these steps:

### 1. Clone the repository
```bash
git clone https://github.com/tonkeeper/opentonapi.git
```

### 2. Build the Docker image
Navigate into the project directory:
```bash
cd opentonapi
```

Then, build the Docker image:
```bash
docker build -t myopentonapi .
```

### 3. Run the Docker container
After the image is built, run the container with the following command:
```bash
docker run -d -p 8081:8081 myopentonapi
```

This command will run the container in the background, exposing port **8081** on your local machine. You can access the API through this port.

Open the following URL in your browser to ensure that the application is running:
```
http://localhost:8081/v2/status
```

You can also [set environment](#environment-variables) variables when running the Docker container. 
```
docker run -d -p 8081:8081 -e LOG_LEVEL=DEBUG myopentonapi
```


## Running from Source Code with Go
To run the project using Go directly from the source code, follow these steps:

### 1. Make sure you have the latest version of Go
Ensure that you have the latest stable version of Go installed on your system. You can check the version of Go using the following command:
```bash
go version
```

### 2. Clone the repository
Clone the OpenTonAPI repository:

```bash
git clone https://github.com/tonkeeper/opentonapi.git
```

Navigate into the project directory:
```bash
cd opentonapi
```

### 3. Download libemulator
For the project to run properly, you need to add **libemulator**, a shared library from the [TON blockchain release repository](https://github.com/ton-blockchain/ton/releases). On a Linux system, follow these steps (on a Windows system, the process is the same, but you must download the appropriate libemulator):


Create the required directory (you can create it wherever you prefer on your system):
```bash
mkdir -p /app/lib
```

Download the `libemulator.so` library (download the library and store it in the folder you created in the previous step):
```bash
wget -O /app/lib/libemulator.so https://github.com/ton-blockchain/ton/releases/download/v2024.08/libemulator-linux-x86_64.so
```

Configure the library path for running the project:
```bash
export LD_LIBRARY_PATH=/app/lib/
```

Run the application
Now, you're ready to run the application using the following Go command:

```bash
go run cmd/api/main.go
```
This command will start the OpenTonAPI and you can access it on the default port **8081**.

Open the following URL in your browser to ensure that the application is running:
```
http://localhost:8081/v2/status
```

You can also [set environment](#environment-variables) variables when start the OpenTonAPI.
```
PORT=8080 LOG_LEVEL=DEBUG go run cmd/api/main.go
```



## Environment Variables

OpenTonAPI supports several environment variables to customize its behavior during startup. These variables allow you to define API configurations, logging, TON Lite Server connections, and other app-level settings. The environment variables are loaded and managed in the [`config.go`](../pkg/config/config.go) file, where you can find detailed parsing logic and default value definitions.



| Environment Variable      | Default Value         | Description                                                                                                           |
|---------------------------|-----------------------|-----------------------------------------------------------------------------------------------------------------------|
| `PORT`                    | `8081`               | Defines the port on which the HTTP API server listens for incoming connections.                                       |
| `LOG_LEVEL`               | `INFO`               | Sets the logging level (`DEBUG`, `INFO`, `WARN`, `ERROR`).                                                            |
| `METRICS_PORT`            | `9010`               | Port used to expose the `/metrics` endpoint for Prometheus metrics.                                                   |
| `LITE_SERVERS`            | `-`                  | A comma-separated list of TON Lite Servers in the format `ip:port:public-key`.                                        |
|                           |                       | Example: `127.0.0.1:14395:6PGkPQSbyFp12esf1NqmDOaLoFA8i9+Mp5+cAx5wtTU=`                                                |
| `SENDING_LITE_SERVERS`    | `-`                  | A comma-separated list of additional Lite Servers dedicated to transaction processing. Format is identical to `LITE_SERVERS`. |
| `IS_TESTNET`              | `false`              | A flag indicating whether the application should operate in testnet mode (`true` or `false`).                         |
| `ACCOUNTS`                | `-`                  | A comma-separated list of account addresses to monitor.                                                              |
| `TON_CONNECT_SECRET`      | `-`                  | Secret used for TonConnect integration.                                                                              |

### Notes  
**`LITE_SERVERS`**: If no Lite Server is set, the application will default to using a random public Lite Server. [`Lite Servers`](https://docs.ton.org/v3/documentation/infra/nodes/node-types) in the TON network are categorized into Full nodes and Archive nodes. If no Lite Server is set, by default, the application may use a Full node Lite Server, which does not provide access to historical data. To access historical data, you need to explicitly set an Archive node. You can find public Archive nodes available for use at the [`global-config.json`](https://ton.org/global-config.json).




## Next Steps
Read [`03_STRUCTURE.md`](./03_STRUCTURE.md), where you will find details about the project structure and the various packages used within the application. It will give you a better understanding of how everything is organized and how the individual packages work together.


