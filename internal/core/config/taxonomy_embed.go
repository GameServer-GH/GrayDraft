package config

import _ "embed"

//go:embed taxonomy_config.yaml
var embeddedTaxonomyYAML []byte

// EmbeddedTaxonomyYAML is the built-in 4D taxonomy config (override via GRAYDRAFT_TAXONOMY_CONFIG).
func EmbeddedTaxonomyYAML() []byte {
	return embeddedTaxonomyYAML
}
