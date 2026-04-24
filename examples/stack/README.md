# appmixer provider — end-to-end stack

Exercises every resource and data source against the in-process mock server.

## Automated (recommended)

From the repo root:

```sh
go test -tags e2e ./e2e/...
```

The harness builds the provider binary, starts the mock server in-process, writes a
`~/.terraformrc`-style dev override into a tempdir, and drives `terraform plan / apply
/ apply-updated / destroy` from Go — asserting on outputs between steps.

## Manual

1. Run the mock server in one terminal:

   ```sh
   go run ./cmd/mockserver
   ```

   It prints its listen address, e.g. `http://127.0.0.1:54321`.

2. In another terminal, export the provider config for the stack:

   ```sh
   export APPMIXER_BASE_URL=http://127.0.0.1:54321
   export APPMIXER_USERNAME=admin@test.com
   export APPMIXER_PASSWORD=test123
   ```

3. Build the provider and wire a dev-override so Terraform uses your local binary:

   ```sh
   go build -o /tmp/terraform-provider-appmixer .
   cat > /tmp/dev.tfrc <<EOF
   provider_installation {
     dev_overrides {
       "ellosoft/appmixer" = "/tmp"
     }
     direct {}
   }
   EOF
   export TF_CLI_CONFIG_FILE=/tmp/dev.tfrc
   ```

4. Run terraform:

   ```sh
   cd examples/stack
   terraform plan
   terraform apply -auto-approve
   terraform apply -auto-approve -var user_password=second-pass   # exercise rotation
   terraform destroy -auto-approve
   ```
