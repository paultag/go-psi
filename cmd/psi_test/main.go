package main

import (
	"fmt"
	"time"

	"pault.ag/go/psi"
)

func main() {
	if err := psi.Monitor(psi.Config{
		Resource:            psi.ResourceCPU,
		Type:                psi.StallTypeSome,
		StallWindowDuration: time.Second / 10,
		WindowDuration:      time.Second,
	}, func() error {
		fmt.Printf("foo\n")
		return nil
	}); err != nil {
		panic(err)
	}
}
