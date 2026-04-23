resource "appmixer_modifiers" "default" {
  document = jsonencode({
    categories = {
      object = { label = "Object", index = 1 }
      list   = { label = "List", index = 2 }
    }
    modifiers = {
      stringify = {
        name        = "stringify"
        label       = "Stringify"
        category    = ["object", "list"]
        description = "Convert an object or list to a JSON string."
        arguments   = [{ name = "space", type = "number", isHash = true }]
        returns     = { type = "string" }
        helperFn    = "function(value, { hash }) { return JSON.stringify(value, null, hash.space); }"
      }
    }
  })
}
