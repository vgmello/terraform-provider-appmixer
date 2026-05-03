package resource_test

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// resourceIDSnapshot captures the Terraform ID of any resource so later steps
// can assert whether the ID changed (replacement) or stayed the same (in-place
// update).
func resourceIDSnapshot(resourceName string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		*dst = rs.Primary.ID
		return nil
	}
}

// resourceIDSame asserts the live ID matches a previously-captured ID (i.e.
// the resource was updated in-place and was not recreated).
func resourceIDSame(resourceName string, prior *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if rs.Primary.ID != *prior {
			return fmt.Errorf("expected id %q to be stable (in-place update), got %q (resource was recreated)", *prior, rs.Primary.ID)
		}
		return nil
	}
}

// resourceIDChanged asserts the live ID differs from a previously-captured ID
// (i.e. the resource was destroyed and re-created).
func resourceIDChanged(resourceName string, prior *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if rs.Primary.ID == *prior {
			return fmt.Errorf("expected id to change after recreation, still %q", rs.Primary.ID)
		}
		return nil
	}
}
