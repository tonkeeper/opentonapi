## Packages and Structure

This project utilizes [`OpenAPI Generator`](https://github.com/ogen-go/ogen) (`ogen`), a tool for generating Go code from OpenAPI v3 specifications, and the [`Tongo`](https://github.com/tonkeeper/tongo) package, a Golang SDK developed by the Tonkeeper team for working with the TON blockchain. These two libraries are central to how the project communicates with the TON blockchain and manages APIs. In addition to these, there are several other packages developed specifically for this project, which we will explain in more detail throughout this document.

## Entry Point
The entry point for running the application is located in the `cmd/api/main.go` file. it's where the application starts when executed.

## Project Structure
The breakdown of the root structure:

- `api`               - Contains configuration files for Ogen to generate API interfaces and other API-related configuration files.
- `cmd`               - Contains the main entry point and execution command for the application.
- `internal`          - Internal packages for utility functions and helpers.
- `pkg`               - Core packages for various functionalities in the application.
- `gen.go`            - Generates code based on the OGEN specification.
- `go.mod`            - Go module dependencies and configuration.
- `go.sum`            - Go checksum dependencies.
- `ogen.yaml`         - OpenAPI configuration for generating code using OGEN.




## pkg Folder
The `pkg` directory contains all the core packages that implement the functionality of this application. Below is an overview of the important packages that exist within the pkg folder:

  - `addressbook`      - This package provides the main functionality for managing the known addresses, jettons, NFT collections, and their manual configurations.
  - `api`              - Handles the core business logic behind the API endpoints for interacting with the system's features, such as querying address information, transactions, and blockchain-related tasks.
  - `app`              - Provides application-level functionalities such as logging configuration.
  - `bath`             -  
  - `blockchain`       - Handles sending payloads and transactions to lite servers and serves as an indexer for retrieving the latest block in real-time.
  - `cache`            - Manages caching mechanisms to enhance performance.
  - `chainstate`       - Manages the state of the blockchain, including tracking APY and managing banned accounts.
  - `config`           - Manages the application's configuration through environment variables, ensuring correct parsing and loading of configurations, including paths for accounts, collections, and jettons.
  - `core`             - 
  - `gasless`          - Handles gasless operations allowing users to perform transactions without the need for TON for gas fees.
  - `image`            - Handles image preview generation by providing functionality to create image URLs with specified dimensions.
  - `litestorage`      - Deals with storage and management of data on LiteServers.
  - `oas`              - Contains utilities related to OpenAPI Specification (OAS) processing.
  - `pusher`           - Handles real-time communication using a pushing mechanism, sending updates and notifications to subscribed users or services.
  - `rates`            - Handles functionality related to fetch, convert, and display market prices for TON tokens across various services.
  - `references`       - 
  - `score`            - 
  - `sentry`           - Manages error tracking and reporting via Sentry.
  - `spam`             - Provides functionality for detecting and filtering spammy or scam-related actions based on predefined rules, specifically for TON transfers, Jetton transfers, and NFT transactions.
  - `testing`          - 
  - `verifier`         - 
  - `wallet`           - 

