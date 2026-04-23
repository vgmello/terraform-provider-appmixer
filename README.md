# terraform-provider-appmixer

Terraform provider for managing an [Appmixer](https://appmixer.com) tenant.

**Status:** Foundation + `appmixer_config` only. More resources coming in follow-up plans.

## Local development (`dev_overrides`)

1. Build the provider locally:
   ```bash
   go build -o terraform-provider-appmixer
   ```
2. Note the absolute path to the resulting binary's directory.
3. Add the following to `~/.terraformrc`:
   ```hcl
   provider_installation {
     dev_overrides {
       "ellosoft/appmixer" = "/absolute/path/to/terraform-provider-appmixer"
     }
     direct {}
   }
   ```
4. In your HCL, reference the provider without declaring a `required_providers` version:
   ```hcl
   provider "appmixer" {
     base_url = "https://api.your-tenant.appmixer.cloud"
     username = "admin@example.com"
     password = var.appmixer_password
   }

   resource "appmixer_config" "jwt" {
     key   = "JWTSecret"
     value = var.jwt_secret
   }
   ```
5. Run `terraform plan` / `terraform apply` — Terraform will print a warning about the override; ignore it during dev.

## Running tests

Unit tests (fast, no external deps):
```bash
go test ./internal/client/ ./internal/provider/
```

Acceptance tests (spawn mock-server subprocess):
```bash
TF_ACC=1 go test ./internal/resource/ -v
```

Requires [Bun](https://bun.sh) installed and `mock-server/node_modules` populated via `cd mock-server && bun install`.
