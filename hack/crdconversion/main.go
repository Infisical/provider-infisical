/*
Copyright 2026 Infisical Inc.
*/

// Command crdconversion configures the conversion webhook of every CRD that
// serves more than one API version. controller-gen does not write
// spec.conversion. Crossplane's package manager fills in the webhook service,
// its port and the CA bundle when it installs the provider.
//
// Usage: go run ./hack/crdconversion <directory with CRD files>
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const conversion = `  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          path: /convert
      conversionReviewVersions:
      - v1
`

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: crdconversion <directory with CRD files>")
		os.Exit(2)
	}
	files, err := filepath.Glob(filepath.Join(os.Args[1], "*.yaml"))
	if err != nil {
		fail(err)
	}
	for _, f := range files {
		if err := configure(f); err != nil {
			fail(fmt.Errorf("%s: %w", f, err))
		}
	}
}

func configure(file string) error {
	raw, err := os.ReadFile(file) //nolint:gosec // the files come from the command line
	if err != nil {
		return err
	}
	var crd struct {
		Spec struct {
			Conversion any `json:"conversion"`
			Versions   []struct {
				Name string `json:"name"`
			} `json:"versions"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(raw, &crd); err != nil {
		return err
	}
	if len(crd.Spec.Versions) < 2 || crd.Spec.Conversion != nil {
		return nil
	}
	content := string(raw)
	if !strings.Contains(content, "\nspec:\n") {
		return fmt.Errorf("cannot find the spec of the CRD")
	}
	content = strings.Replace(content, "\nspec:\n", "\nspec:\n"+conversion, 1)
	return os.WriteFile(file, []byte(content), 0o644) //nolint:gosec // CRD files are not secret
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
