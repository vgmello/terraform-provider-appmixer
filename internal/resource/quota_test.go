package resource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const quotaSourceV1 = `module.exports = { rules: [{ limit: 100, window: 60000, resource: 'requests' }] };`
const quotaSourceV2 = `module.exports = { rules: [{ limit: 200, window: 60000, resource: 'requests' }] };`

func TestAccQuota_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_quota" "q" {
  service_id = "appmixer:hubspot"
  source     = "` + quotaSourceV1 + `"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_quota.q", "service_id", "appmixer:hubspot"),
					resource.TestCheckResourceAttrSet("appmixer_quota.q", "id"),
					resource.TestCheckResourceAttr("appmixer_quota.q", "is_custom", "true"),
				),
			},
		},
	})
}

func TestAccQuota_updateSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_quota" "u" {
  service_id = "tenant:update-me"
  source     = "` + quotaSourceV1 + `"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_quota.u", "source", quotaSourceV1),
			},
			{
				Config: `
resource "appmixer_quota" "u" {
  service_id = "tenant:update-me"
  source     = "` + quotaSourceV2 + `"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_quota.u", "source", quotaSourceV2),
					resource.TestCheckResourceAttr("appmixer_quota.u", "service_id", "tenant:update-me"),
				),
			},
		},
	})
}

func TestAccQuota_replaceOnServiceIDChange(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_quota" "r" {
  service_id = "tenant:first-name"
  source     = "` + quotaSourceV1 + `"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_quota.r", "service_id", "tenant:first-name"),
			},
			{
				Config: `
resource "appmixer_quota" "r" {
  service_id = "tenant:second-name"
  source     = "` + quotaSourceV1 + `"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_quota.r", "service_id", "tenant:second-name"),
			},
		},
	})
}

func TestAccQuota_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_quota" "i" {
  service_id = "tenant:imported"
  source     = "` + quotaSourceV1 + `"
}
`,
			},
			{
				ResourceName:      "appmixer_quota.i",
				ImportState:       true,
				ImportStateId:     "tenant:imported",
				ImportStateVerify: true,
			},
		},
	})
}
