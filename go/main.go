package main

import (
	"context"
	"os"

	"github.com/gardener/vpn2/ippool"
)

func main() {
	broker, err := ippool.NewIPAddressBrokerFromEnv()
	if err != nil {
		panic(err)
	}

	ctx := context.TODO()
	ip, err := broker.AcquireIP(ctx)
	if err != nil {
		panic(err)
	}
	if output := os.Getenv("OUTPUT"); output != "" {
		err = os.WriteFile(output, []byte(ip), 0420)
		if err != nil {
			panic(err)
		}
	}
}
