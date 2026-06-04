package prompt

import "testing"

func TestVariablesReturnsSortedUniquePaths(t *testing.T) {
	got := Variables("{{role.goal}} {{ role.name }} {{role.goal}} {{context.language}}")

	want := []string{"context.language", "role.goal", "role.name"}
	if len(got) != len(want) {
		t.Fatalf("expected %d variables, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected variable %q at index %d, got %q", want[i], i, got[i])
		}
	}
}

func TestRenderReplacesVariablesAndKeepsPlainText(t *testing.T) {
	template := "你是{{role.name}}，请使用{{context.language}}回答。"
	values := map[string]string{
		"role.name":        "学习助理",
		"context.language": "中文",
	}

	got, err := Render(template, values)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if got != "你是学习助理，请使用中文回答。" {
		t.Fatalf("unexpected rendered output: %q", got)
	}
}

func TestValidateRejectsMissingVariable(t *testing.T) {
	err := Validate("{{role.name}} {{role.goal}}", map[string]string{
		"role.name": "学习助理",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRejectsEmptyVariable(t *testing.T) {
	err := Validate("{{role.name}}", map[string]string{
		"role.name": "   ",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
