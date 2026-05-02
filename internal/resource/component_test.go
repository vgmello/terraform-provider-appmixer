package resource_test

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// createTestComponentZip creates a temporary zip file with a valid directory
// structure for testing. The selector (e.g. "appmixer.test") is converted to
// a directory path (appmixer/test/) containing a minimal component.json.
func createTestComponentZip(t *testing.T, selector string) string {
	t.Helper()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "component.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)

	// Convert dot-separated selector to path: "appmixer.test" -> "appmixer/test/"
	dirPath := strings.ReplaceAll(selector, ".", "/")
	entryPath := dirPath + "/component.json"

	fw, err := w.Create(entryPath)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := fw.Write([]byte(`{"label": "Test"}`)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return zipPath
}

// createTestComponentZipV2 creates a temporary zip with the same structure but
// different content so that the file hash changes between test steps.
func createTestComponentZipV2(t *testing.T, selector string) string {
	t.Helper()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "component-v2.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)

	dirPath := strings.ReplaceAll(selector, ".", "/")
	entryPath := dirPath + "/component.json"

	fw, err := w.Create(entryPath)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := fw.Write([]byte(`{"label": "Test V2"}`)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return zipPath
}

// componentIDSnapshot captures the id of an appmixer_component resource so
// later steps can assert the id is unchanged or changed.
func componentIDSnapshot(resourceName string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		*dst = rs.Primary.ID
		return nil
	}
}

// componentIDSame asserts the live id matches a previously-captured id.
func componentIDSame(resourceName string, prior *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if rs.Primary.ID != *prior {
			return fmt.Errorf("expected id %q to be stable, got %q (resource was recreated)", *prior, rs.Primary.ID)
		}
		return nil
	}
}

// componentIDChanged asserts the live id differs from a previously-captured id
// (i.e. recreation happened).
func componentIDChanged(resourceName string, prior *string) resource.TestCheckFunc {
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

// componentAttrSnapshot captures the value of an attribute in state.
func componentAttrSnapshot(resourceName, attr string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		v, ok := rs.Primary.Attributes[attr]
		if !ok {
			return fmt.Errorf("attribute %s not found on %s", attr, resourceName)
		}
		*dst = v
		return nil
	}
}

// componentAttrChanged asserts the value of an attribute differs from a
// previously-captured value.
func componentAttrChanged(resourceName, attr string, prior *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		v, ok := rs.Primary.Attributes[attr]
		if !ok {
			return fmt.Errorf("attribute %s not found on %s", attr, resourceName)
		}
		if v == *prior {
			return fmt.Errorf("expected %s to change, still %q", attr, v)
		}
		return nil
	}
}

func componentConfig(selector, zipPath string) string {
	return fmt.Sprintf(`
resource "appmixer_component" "test" {
  selector = %q
  source   = %q
}
`, selector, zipPath)
}

func componentConfigWithReplaceAll(selector, zipPath string, replaceAll bool) string {
	return fmt.Sprintf(`
resource "appmixer_component" "test" {
  selector    = %q
  source      = %q
  replace_all = %t
}
`, selector, zipPath, replaceAll)
}

func TestAccComponent_basic(t *testing.T) {
	zipPath := createTestComponentZip(t, "appmixer.test")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: componentConfig("appmixer.test", zipPath),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_component.test", "id"),
					resource.TestCheckResourceAttr("appmixer_component.test", "selector", "appmixer.test"),
					resource.TestCheckResourceAttrSet("appmixer_component.test", "file_hash"),
					resource.TestCheckResourceAttrSet("appmixer_component.test", "published_at"),
				),
			},
		},
	})
}

func TestAccComponent_update(t *testing.T) {
	selector := "appmixer.update"
	zipPathV1 := createTestComponentZip(t, selector)
	zipPathV2 := createTestComponentZipV2(t, selector)

	var priorID string
	var priorHash string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: componentConfig(selector, zipPathV1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_component.test", "id"),
					componentIDSnapshot("appmixer_component.test", &priorID),
					componentAttrSnapshot("appmixer_component.test", "file_hash", &priorHash),
				),
			},
			{
				Config: componentConfig(selector, zipPathV2),
				Check: resource.ComposeTestCheckFunc(
					componentIDSame("appmixer_component.test", &priorID),
					componentAttrChanged("appmixer_component.test", "file_hash", &priorHash),
					resource.TestCheckResourceAttrSet("appmixer_component.test", "published_at"),
				),
			},
		},
	})
}

func TestAccComponent_replaceAll(t *testing.T) {
	zipPath := createTestComponentZip(t, "appmixer.replaceall")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: componentConfigWithReplaceAll("appmixer.replaceall", zipPath, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_component.test", "id"),
					resource.TestCheckResourceAttr("appmixer_component.test", "replace_all", "true"),
					resource.TestCheckResourceAttr("appmixer_component.test", "selector", "appmixer.replaceall"),
				),
			},
		},
	})
}

func TestAccComponent_replaceOnSelectorChange(t *testing.T) {
	zipPath1 := createTestComponentZip(t, "appmixer.test1")
	zipPath2 := createTestComponentZip(t, "appmixer.test2")

	var priorID string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: componentConfig("appmixer.test1", zipPath1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_component.test", "selector", "appmixer.test1"),
					componentIDSnapshot("appmixer_component.test", &priorID),
				),
			},
			{
				Config: componentConfig("appmixer.test2", zipPath2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_component.test", "selector", "appmixer.test2"),
					componentIDChanged("appmixer_component.test", &priorID),
				),
			},
		},
	})
}

func TestAccComponent_import(t *testing.T) {
	zipPath := createTestComponentZip(t, "appmixer.importtest")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: componentConfig("appmixer.importtest", zipPath),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_component.test", "id"),
				),
			},
			{
				ResourceName:            "appmixer_component.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source", "file_hash", "published_at", "replace_all"},
			},
		},
	})
}
