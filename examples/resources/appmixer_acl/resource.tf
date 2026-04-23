resource "appmixer_acl" "example" {
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
