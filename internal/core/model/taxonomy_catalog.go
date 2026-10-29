package model

import (
	"os"
	"strings"
	"sync"

	"github.com/graydraft/core/config"
	"gopkg.in/yaml.v3"
)

// taxonomyFile is the on-disk YAML shape.
type taxonomyFile struct {
	DefaultDomain string                      `yaml:"default_domain"`
	Domains       map[string]domainTaxonomyYAML `yaml:"domains"`
}

type domainTaxonomyYAML struct {
	Aliases        []string            `yaml:"aliases"`
	DetectKeywords []string            `yaml:"detect_keywords"`
	Scenes         []LabelDefinition   `yaml:"scenes"`
	Styles         []LabelDefinition   `yaml:"styles"`
}

// UniversalDomainKey is the industry-agnostic cold-start domain in taxonomy_config.yaml.
const UniversalDomainKey = "universal"

// Catalog holds all domain taxonomies loaded from config.
type Catalog struct {
	defaultKey string
	byDomain   map[string]*DomainTaxonomy
}

var (
	catalogOnce sync.Once
	catalogInst *Catalog
	catalogErr  error
)

// DefaultCatalog returns the process-wide taxonomy catalog (lazy-loaded).
func DefaultCatalog() *Catalog {
	catalogOnce.Do(func() {
		catalogInst, catalogErr = LoadTaxonomyCatalog("")
		if catalogErr != nil {
			catalogInst = fallbackCatalog()
		}
	})
	if catalogInst == nil {
		return fallbackCatalog()
	}
	return catalogInst
}

// LoadTaxonomyCatalog reads taxonomy_config.yaml (embedded default, optional file override).
func LoadTaxonomyCatalog(path string) (*Catalog, error) {
	raw := config.EmbeddedTaxonomyYAML()
	if path == "" {
		path = strings.TrimSpace(os.Getenv("GRAYDRAFT_TAXONOMY_CONFIG"))
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	var file taxonomyFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	defKey := strings.TrimSpace(file.DefaultDomain)
	if defKey == "" {
		defKey = "default"
	}
	out := &Catalog{defaultKey: defKey, byDomain: make(map[string]*DomainTaxonomy, len(file.Domains))}
	for name, spec := range file.Domains {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		out.byDomain[key] = &DomainTaxonomy{
			Domain:         key,
			Aliases:        append([]string(nil), spec.Aliases...),
			DetectKeywords: append([]string(nil), spec.DetectKeywords...),
			Scenes:         append([]LabelDefinition(nil), spec.Scenes...),
			Styles:         append([]LabelDefinition(nil), spec.Styles...),
		}
	}
	if _, ok := out.byDomain[defKey]; !ok {
		if _, ok2 := out.byDomain["default"]; ok2 {
			out.defaultKey = "default"
		} else if len(out.byDomain) > 0 {
			for k := range out.byDomain {
				out.defaultKey = k
				break
			}
		}
	}
	return out, nil
}

func fallbackCatalog() *Catalog {
	c, _ := LoadTaxonomyCatalogFromBytes(config.EmbeddedTaxonomyYAML())
	if c != nil {
		return c
	}
	return &Catalog{
		defaultKey: "default",
		byDomain: map[string]*DomainTaxonomy{
			"default": {Domain: "default"},
		},
	}
}

// LoadTaxonomyCatalogFromBytes parses YAML (tests).
func LoadTaxonomyCatalogFromBytes(raw []byte) (*Catalog, error) {
	var file taxonomyFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	defKey := strings.TrimSpace(file.DefaultDomain)
	if defKey == "" {
		defKey = "default"
	}
	out := &Catalog{defaultKey: defKey, byDomain: make(map[string]*DomainTaxonomy)}
	for name, spec := range file.Domains {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		out.byDomain[key] = &DomainTaxonomy{
			Domain:         key,
			Aliases:        append([]string(nil), spec.Aliases...),
			DetectKeywords: append([]string(nil), spec.DetectKeywords...),
			Scenes:         append([]LabelDefinition(nil), spec.Scenes...),
			Styles:         append([]LabelDefinition(nil), spec.Styles...),
		}
	}
	return out, nil
}

// Default returns the fallback domain taxonomy.
func (c *Catalog) Default() *DomainTaxonomy {
	if c == nil {
		return nil
	}
	if t := c.byDomain[c.defaultKey]; t != nil {
		return t
	}
	for _, t := range c.byDomain {
		return t
	}
	return nil
}

// Get returns taxonomy for a domain key, or default if unknown.
func (c *Catalog) Get(domainKey string) *DomainTaxonomy {
	if c == nil {
		return nil
	}
	key := strings.TrimSpace(domainKey)
	if key != "" {
		if t, ok := c.byDomain[key]; ok {
			return t
		}
	}
	return c.Default()
}

// Has reports whether a domain key exists in the catalog.
func (c *Catalog) Has(domainKey string) bool {
	if c == nil {
		return false
	}
	_, ok := c.byDomain[strings.TrimSpace(domainKey)]
	return ok
}

// DefaultDomainKey is the configured fallback domain name.
func (c *Catalog) DefaultDomainKey() string {
	if c == nil || c.defaultKey == "" {
		return "default"
	}
	return c.defaultKey
}

// InferDomainFromText picks the best domain key by aliases/keywords in free text.
func (c *Catalog) InferDomainFromText(text string) string {
	if c == nil {
		return ""
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	best := ""
	bestScore := 0
	for key, tax := range c.byDomain {
		score := 0
		for _, alias := range tax.Aliases {
			if alias == "" {
				continue
			}
			if len([]rune(alias)) < 3 {
				continue
			}
			if strings.Contains(lower, strings.ToLower(alias)) {
				score += 2
			}
		}
		for _, kw := range tax.DetectKeywords {
			if kw != "" && strings.Contains(lower, strings.ToLower(kw)) {
				score++
			}
		}
		if strings.Contains(lower, strings.ToLower(key)) {
			score += 3
		}
		if score > bestScore {
			bestScore = score
			best = key
		}
	}
	return best
}

// InferDomainFromQuery scans the user query across all domain taxonomies.
func (c *Catalog) InferDomainFromQuery(query string) string {
	return c.InferDomainFromText(query)
}

// MatchDomainKeyInText returns the longest catalog domain key contained in text (e.g. 学科资料 in profile).
func (c *Catalog) MatchDomainKeyInText(text string) string {
	if c == nil {
		return ""
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	best := ""
	bestLen := 0
	for key := range c.byDomain {
		if key == c.defaultKey || key == "" {
			continue
		}
		if !strings.Contains(text, key) {
			continue
		}
		if n := len([]rune(key)); n > bestLen {
			bestLen = n
			best = key
		}
	}
	return best
}

// ResolveKey picks taxonomy domain from explicit hint or falls back to universal scaffold (no industry inference).
func (c *Catalog) ResolveKey(profile *ContextProfile, workspaceContext string, domainHint string) string {
	if c == nil {
		return UniversalDomainKey
	}
	if hint := strings.TrimSpace(domainHint); hint != "" {
		if c.Has(hint) {
			return hint
		}
	}
	// Industry-specific InferDomainFromText removed: Blueprint / WorkspacePendingTaxonomy own domain naming.
	return c.DefaultDomainKey()
}

// ResolveTaxonomy is a convenience wrapper for ResolveKey + Get.
func (c *Catalog) ResolveTaxonomy(profile *ContextProfile, workspaceContext string, domainHint string) *DomainTaxonomy {
	return c.Get(c.ResolveKey(profile, workspaceContext, domainHint))
}
