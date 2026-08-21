# Publish a component from a zip file built elsewhere in the repository.
resource "appmixer_component" "invoice_parser" {
  selector = "acme.billing.InvoiceParser"
  source   = "${path.module}/dist/invoice-parser.zip"
}

# Replace every previously published version of the component instead of
# adding a new one alongside them.
resource "appmixer_component" "legacy_exporter" {
  selector    = "acme.billing.LegacyExporter"
  source      = "${path.module}/dist/legacy-exporter.zip"
  replace_all = true
}
