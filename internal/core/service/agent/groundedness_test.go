package agent

import "testing"

func TestEvaluateGroundedness(t *testing.T) {
	r := EvaluateGroundedness("版本是 1.2.3。", []string{"当前版本是 1.2.3"}, 1, 1)
	if r.Groundedness < .62 || r.ShouldRefuse {
		t.Fatalf("%+v", r)
	}
	r = EvaluateGroundedness("装备隐藏属性是无限大。", []string{"当前版本是 1.2.3"}, 0, 1)
	if !r.ShouldRefuse {
		t.Fatalf("%+v", r)
	}
}
