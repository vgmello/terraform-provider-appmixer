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
  name   = "appmixer:hubspot"
  source = "` + quotaSourceV1 + `"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_quota.q", "name", "appmixer:hubspot"),
					resource.TestCheckResourceAttr("appmixer_quota.q", "id", "appmixer:hubspot"),
					resource.TestCheckResourceAttr("appmixer_quota.q", "is_custom", "true"),
					resource.TestCheckResourceAttrSet("appmixer_quota.q", "quota_id"),
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
  name   = "tenant:update-me"
  source = "` + quotaSourceV1 + `"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_quota.u", "source", quotaSourceV1),
			},
			{
				Config: `
resource "appmixer_quota" "u" {
  name   = "tenant:update-me"
  source = "` + quotaSourceV2 + `"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_quota.u", "source", quotaSourceV2),
					resource.TestCheckResourceAttr("appmixer_quota.u", "name", "tenant:update-me"),
				),
			},
		},
	})
}

func TestAccQuota_replaceOnNameChange(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_quota" "r" {
  name   = "tenant:first-name"
  source = "` + quotaSourceV1 + `"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_quota.r", "name", "tenant:first-name"),
			},
			{
				Config: `
resource "appmixer_quota" "r" {
  name   = "tenant:second-name"
  source = "` + quotaSourceV1 + `"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_quota.r", "name", "tenant:second-name"),
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
  name   = "tenant:imported"
  source = "` + quotaSourceV1 + `"
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
