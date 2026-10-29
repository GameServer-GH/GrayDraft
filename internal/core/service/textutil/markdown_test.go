package textutil

import "testing"

func TestNormalizeKBMarkdown_userExample(t *testing.T) {
	in := "###新手快速升级指南1.**优先推主线，别急着做支线**-主线任务是前期经验的主要来源，能最快解锁新地图和功能。2.**集中资源，单核培养**-新手资源有限，不要把经验分散给多只宠物。**把所有经验果优先喂给你的主力宠物**（例如选初始宠时的火花），把它等级拉起来后，就能轻松带其他小宠升级。3.**利用属性克制练级**-战斗中利用属性克制机制可以更快结束战斗，获得更多经验。基本克制关系为：**火克草、草克水、水克火**。4.**别忘了每日任务**-每日任务如魔法学院的“学院试炼”、完成“送信任务”等，是稳定的经验果和道具来源。"
	got := NormalizeKBMarkdown(in)
	if !stringsContains(got, "### 新手快速升级指南") {
		t.Fatalf("missing spaced heading: %q", got)
	}
	if !stringsContains(got, "1. **优先推主线") {
		t.Fatalf("missing numbered item break: %q", got)
	}
	if !stringsContains(got, "**\n- ") {
		t.Fatalf("missing bullet line after bold: %q", got)
	}
	if !stringsContains(got, "\n\n2.") {
		t.Fatalf("missing break before item 2: %q", got)
	}
}

func TestNormalizeKBMarkdown_dateRangeOnOneLine(t *testing.T) {
	in := "文本编码的出没时间为**06.19 - 06.21**（版本发布）。"
	got := NormalizeKBMarkdown(in)
	if stringsContains(got, "06. 19") || stringsContains(got, "\n-\n") || stringsContains(got, "\n - ") {
		t.Fatalf("date range should stay inline: %q", got)
	}
	if !stringsContains(got, "06.19 - 06.21") {
		t.Fatalf("missing intact date range: %q", got)
	}
}

func TestNormalizeKBMarkdown_gluedTable(t *testing.T) {
	in := "### 番茄在减脂餐中的出现时间与分量|星期|餐次|食材及分量||------|------|------------||周一|早餐|小番茄100g||周二|晚餐|番茄100g（番茄金针菇汤）||周日|晚餐|番茄100g（番茄蔬菜汤）|"
	got := NormalizeKBMarkdown(in)
	for _, sub := range []string{
		"### 番茄在减脂餐中的出现时间与分量\n\n| 星期 | 餐次 | 食材及分量 |",
		"| --- | --- | --- |",
		"| 周一 | 早餐 | 小番茄100g |",
		"| 周二 | 晚餐 | 番茄100g（番茄金针菇汤） |",
		"| 周日 | 晚餐 | 番茄100g（番茄蔬菜汤） |",
	} {
		if !stringsContains(got, sub) {
			t.Fatalf("normalized table missing %q:\n%s", sub, got)
		}
	}
	if stringsContains(got, "||") {
		t.Fatalf("table still contains glued row marker: %q", got)
	}
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexString(s, sub) >= 0)
}

func indexString(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestNormalizeKBMarkdown_bulletBeforeBold verifies the fix for KB-only LLM output where
// bullet items with bold labels are glued to the heading or previous sentence.
func TestNormalizeKBMarkdown_bulletBeforeBold(t *testing.T) {
	in := "### 样例模型引用说明-**捕捉地点**：交汇镇。-**解锁条件**：连续捕捉公式与代码。-**推荐引用标记**：参考资料未提供具体引用标记信息。"
	got := NormalizeKBMarkdown(in)
	if !stringsContains(got, "\n- **捕捉地点**") {
		t.Fatalf("missing line break before bullet+bold: %q", got)
	}
	if !stringsContains(got, "\n- **解锁条件**") {
		t.Fatalf("missing line break before second bullet+bold: %q", got)
	}
	if !stringsContains(got, "\n- **推荐引用标记**") {
		t.Fatalf("missing line break before third bullet+bold: %q", got)
	}
}
