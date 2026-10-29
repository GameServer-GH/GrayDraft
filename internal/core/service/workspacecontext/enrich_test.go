package workspacecontext

import "testing"

func Test_parseJSONFromLLM(t *testing.T) {
	var v enrichLLMResponse
	raw := `说明文字
{"description":"InternLM 教程与 API 说明","entities":["模型"],"rules":["忽略视觉描述"],"negative_context":["广告"]}`
	if err := parseJSONFromLLM(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v.Description == "" || len(v.Entities) != 1 {
		t.Fatalf("%+v", v)
	}
}
