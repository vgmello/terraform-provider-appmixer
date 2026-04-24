// Command mockserver runs the Appmixer mock API on a random port and prints
// its address. Used for manual end-to-end runs of the provider against the
// mock. The in-process test harness uses mockserver.Start directly.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ellosoft/terraform-provider-appmixer/internal/mockserver"
)

func main() {
	addr, stop := mockserver.Start()
	defer stop()

	fmt.Println(addr)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
