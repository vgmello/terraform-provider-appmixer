# Authoritative (default): this resource owns the entire list for `type`.
# Any rules configured out-of-band are wiped on apply; destroy clears the list.
resource "appmixer_acl" "components" {
  type = "components"
  rules = [
    {
      role       = "admin"
      resource   = "*"
      action     = ["read", "write", "delete"]
      attributes = ["*"]
    },
    {
      role       = "viewer"
      resource   = "flow"
      action     = ["read"]
      attributes = []
    }
  ]
}

# Merge: this resource owns only the rules it declares. Rules added in the
# Appmixer UI (or by other automation) are preserved across apply and destroy.
# Rules removed from this block between applies are deleted from the server.
resource "appmixer_acl" "routes" {
  type = "routes"
  mode = "merge"
  rules = [
    {
      role       = "ci-deployer"
      resource   = "deploy/*"
      action     = ["write"]
      attributes = ["*"]
    }
  ]
}
