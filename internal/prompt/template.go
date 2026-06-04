package prompt

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// Variables 返回模板中引用到的变量列表，按字典序去重输出。
func Variables(template string) []string {
	matches := variablePattern.FindAllStringSubmatch(template, -1)
	if len(matches) == 0 {
		return nil
	}

	unique := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		unique[match[1]] = struct{}{}
	}

	var variables []string
	for variable := range unique {
		variables = append(variables, variable)
	}
	sort.Strings(variables)
	return variables
}

// Validate 校验模板和变量是否满足纯替换语义的要求。
func Validate(template string, values map[string]string) error {
	trimmedTemplate := strings.TrimSpace(template)
	if trimmedTemplate == "" {
		return fmt.Errorf("template is empty")
	}

	for _, variable := range Variables(template) {
		value, ok := values[variable]
		if !ok {
			return fmt.Errorf("missing variable %q", variable)
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("variable %q is empty", variable)
		}
	}

	return nil
}

// Render 执行模板变量替换并返回最终 prompt。
func Render(template string, values map[string]string) (string, error) {
	if err := Validate(template, values); err != nil {
		return "", err
	}

	rendered := variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		captures := variablePattern.FindStringSubmatch(match)
		if len(captures) < 2 {
			return match
		}
		return values[captures[1]]
	})

	return rendered, nil
}
