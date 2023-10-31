package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tonkeeper/opentonapi/tonapi"
)

const myAccount = "EQBszTJahYw3lpP64ryqscKQaDGk4QpsO7RO6LYVvKHSINS0"

func printAccountInformation(myAccount string) error {
	client, err := tonapi.New()
	if err != nil {
		return err
	}
	account, err := client.GetAccount(context.Background(), tonapi.GetAccountParams{AccountID: myAccount})
	if err != nil {
		return err
	}
	fmt.Printf("Account: %v\n", account.Balance)

	balances, err := client.GetAccountJettonsBalances(context.Background(), tonapi.GetAccountJettonsBalancesParams{
		AccountID: myAccount,
	})
	if err != nil {
		return err
	}
	for _, balance := range balances.Balances {
		fmt.Printf("Jetton: %v, balance: %v\n", balance.Jetton.Name, balance.Balance)
	}

	items, err := client.GetAccountNftItems(context.Background(), tonapi.GetAccountNftItemsParams{
		AccountID: myAccount,
	})
	if err != nil {
		return err
	}
	for _, item := range items.NftItems {
		fmt.Printf("NFTs: %v\n", item.Metadata)
	}
	return nil
}

func main() {
	if err := printAccountInformation(myAccount); err != nil {
		log.Fatalf("error: %v", err)
	}
}
