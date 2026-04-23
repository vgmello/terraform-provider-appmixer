package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/ellosoft/terraform-provider-appmixer/internal/provider"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Run with support for attaching a debugger")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(), providerserver.ServeOpts{
		Address: "registry.terraform.io/ellosoft/appmixer",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
