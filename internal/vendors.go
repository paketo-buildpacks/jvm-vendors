package internal

import (
	"fmt"
	"strings"

	"github.com/paketo-buildpacks/libpak/v2"
)

func LoadJvmVendors(cr *libpak.ConfigurationResolver) ([]string, error) {
	jvmVendorsRaw, _ := cr.Resolve("BP_JVM_VENDORS")
	if jvmVendorsRaw == "" {
		return []string{}, fmt.Errorf("BP_JVM_VENDORS is empty")
	}

	jvmVendors := []string{}
	for _, jvmVendorDirty := range strings.Split(jvmVendorsRaw, ",") {
		jvmVendor := strings.TrimSpace(jvmVendorDirty)
		if jvmVendor != "" {
			jvmVendors = append(jvmVendors, jvmVendor)
		}
	}

	return jvmVendors, nil
}
