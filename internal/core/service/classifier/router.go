package classifier

// ApplyUserOverrides keeps explicit user weights when provided.
func ApplyUserOverrides(plan RoutePlan, ftOverride, vecOverride *float64, useGlobal *bool) RoutePlan {
	if ftOverride != nil {
		plan.FulltextWeight = *ftOverride
	}
	if vecOverride != nil {
		plan.VectorWeight = *vecOverride
	}
	if useGlobal != nil {
		plan.UseGlobalIndex = *useGlobal
	}
	normalizeWeights(&plan)
	return plan
}
