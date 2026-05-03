package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/ellosoft/terraform-provider-appmixer/internal/provider"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// The goreleaser configuration injects the release version automatically;
// local builds default to "dev".
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Run with support for attaching a debugger")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/ellosoft/appmixer",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
