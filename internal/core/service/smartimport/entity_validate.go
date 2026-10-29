// Package smartimport provides entity validation against taxonomy alias libraries.
// Implements three-step entity validation (plan 2.2): alias lookup → direct match → embedding fallback.
package smartimport

import (
	"context"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/taxonomyevolve"
)

// EntityValidator validates entities against domain taxonomy.
type EntityValidator struct {
	tax *model.DomainTaxonomy
}

// NewEntityValidator creates a validator from workspace taxonomy.
func NewEntityValidator(hintKBType, workspaceBackground string) *EntityValidator {
	ctx := context.Background()
	tax := taxonomyevolve.Effective(ctx, hintKBType, nil, workspaceBackground)
	return &EntityValidator{tax: tax}
}

// ValidateEntities performs three-step validation on LLM-extracted entities.
// Step 1: Check against taxonomy aliases and detect keywords.
// Step 2: Check if entity appears as substring in source content.
// Step 3: Deferred to embedding (not implemented inline — caller handles if needed).
//
// Returns only validated entities and the ratio of validated/total.
func (v *EntityValidator) ValidateEntities(entities []string, content string) ([]string, float64) {
	if len(entities) == 0 {
		return nil, 1.0
	}

	var validated []string
	for _, e := range entities {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}

		// Step 1: taxonomy alias / keyword hit
		if v.inTaxonomy(e) {
			validated = append(validated, e)
			continue
		}

		// Step 2: direct match in content
		if strings.Contains(content, e) {
			validated = append(validated, e)
			continue
		}

		// Step 3: not validated — drop
	}

	ratio := 0.0
	if len(entities) > 0 {
		ratio = float64(len(validated)) / float64(len(entities))
	}
	return validated, ratio
}

// inTaxonomy checks if an entity is recognized in the domain taxonomy.
func (v *EntityValidator) inTaxonomy(entity string) bool {
	if v.tax == nil {
		return false
	}
	// Check aliases
	for _, alias := range v.tax.Aliases {
		if strings.EqualFold(alias, entity) {
			return true
		}
	}
	// Check detect keywords
	for _, kw := range v.tax.DetectKeywords {
		if strings.Contains(kw, entity) || strings.Contains(entity, kw) {
			return true
		}
	}
	return false
}

// ValidateEntityList is a convenience wrapper that creates a validator and runs validation.
// Returns the validated entities and the trace ratio for confidence checks.
func ValidateEntityList(entities []string, content, hintKBType, workspaceBackground string) ([]string, float64) {
	if len(entities) == 0 {
		return nil, 1.0
	}
	v := NewEntityValidator(hintKBType, workspaceBackground)
	return v.ValidateEntities(entities, content)
}

// DomainKeywords returns domain keywords with IDF-like weights for the info density provider.
func (v *EntityValidator) DomainKeywords() map[string]float64 {
	if v.tax == nil || len(v.tax.DetectKeywords) == 0 {
		return nil
	}
	kw := make(map[string]float64, len(v.tax.DetectKeywords))
	for i, k := range v.tax.DetectKeywords {
		// Earlier keywords tend to be more important; weight decays
		weight := 0.7 - float64(i)*0.02
		if weight < 0.3 {
			weight = 0.3
		}
		kw[strings.TrimSpace(k)] = weight
	}
	return kw
}

// TaxonomyDomainTermProvider adapts EntityValidator to the DomainTermProvider interface
// for use with CalcInfoDensityWithTerms.
type TaxonomyDomainTermProvider struct {
	validator   *EntityValidator
	fallback    DomainTermProvider
}

func (p TaxonomyDomainTermProvider) DomainKeywords() map[string]float64 {
	kw := p.validator.DomainKeywords()
	if len(kw) > 0 {
		return kw
	}
	if p.fallback != nil {
		return p.fallback.DomainKeywords()
	}
	return defaultTermProvider{}.DomainKeywords()
}

// NewDomainTermProvider creates a provider that prefers taxonomy keywords
// and falls back to the default cross-domain provider.
func NewDomainTermProvider(hintKBType, workspaceBackground string) DomainTermProvider {
	v := NewEntityValidator(hintKBType, workspaceBackground)
	return TaxonomyDomainTermProvider{validator: v, fallback: defaultTermProvider{}}
}
