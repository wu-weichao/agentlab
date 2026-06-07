package tools

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// CalculatorTool 执行基础数学表达式计算。
// 它使用手写表达式解析器，只支持数字、括号和 + - * /，不执行任何脚本或 Go 代码。
type CalculatorTool struct{}

// NewCalculatorTool 创建 calculator 工具。
func NewCalculatorTool() *CalculatorTool {
	return &CalculatorTool{}
}

func (t *CalculatorTool) Name() string {
	return "calculator"
}

func (t *CalculatorTool) Description() string {
	return "执行安全的基础数学表达式计算"
}

func (t *CalculatorTool) Parameters() []Parameter {
	return []Parameter{{
		Name:        "expression",
		Type:        "string",
		Required:    true,
		Description: "只包含数字、括号和 + - * / 的基础数学表达式",
	}}
}

// Execute 校验表达式并返回确定性计算结果。
func (t *CalculatorTool) Execute(_ context.Context, args map[string]any) ToolResult {
	expression, ok := requiredString(args, "expression")
	if !ok {
		return errorResult("expression is required", map[string]any{"code": "invalid_arguments"})
	}

	value, err := evalExpression(expression)
	if err != nil {
		return errorResult(err.Error(), map[string]any{
			"code":       "invalid_expression",
			"expression": expression,
		})
	}

	content := strconv.FormatFloat(value, 'f', -1, 64)
	return successResult(content, map[string]any{
		"expression": expression,
		"result":     content,
	})
}

type expressionParser struct {
	input string
	pos   int
}

// evalExpression 使用递归下降解析基础四则表达式。
// 这里不用表达式库或脚本引擎，是为了把可执行语义限制在纯数学运算内。
func evalExpression(input string) (float64, error) {
	parser := &expressionParser{input: input}
	value, err := parser.parseExpression()
	if err != nil {
		return 0, err
	}
	parser.skipSpaces()
	if parser.pos != len(parser.input) {
		return 0, fmt.Errorf("unexpected token %q", parser.input[parser.pos:])
	}
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, fmt.Errorf("invalid calculation result")
	}
	return value, nil
}

func (p *expressionParser) parseExpression() (float64, error) {
	value, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.match('+') {
			next, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value += next
			continue
		}
		if p.match('-') {
			next, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value -= next
			continue
		}
		return value, nil
	}
}

func (p *expressionParser) parseTerm() (float64, error) {
	value, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.match('*') {
			next, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			value *= next
			continue
		}
		if p.match('/') {
			next, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if next == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			value /= next
			continue
		}
		return value, nil
	}
}

func (p *expressionParser) parseFactor() (float64, error) {
	p.skipSpaces()
	if p.match('+') {
		return p.parseFactor()
	}
	if p.match('-') {
		value, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -value, nil
	}
	if p.match('(') {
		value, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if !p.match(')') {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		return value, nil
	}
	return p.parseNumber()
}

func (p *expressionParser) parseNumber() (float64, error) {
	p.skipSpaces()
	start := p.pos
	hasDot := false
	for p.pos < len(p.input) {
		r := rune(p.input[p.pos])
		if unicode.IsDigit(r) {
			p.pos++
			continue
		}
		if r == '.' && !hasDot {
			hasDot = true
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		return 0, fmt.Errorf("expected number")
	}
	text := p.input[start:p.pos]
	if strings.Count(text, ".") > 1 {
		return 0, fmt.Errorf("invalid number %q", text)
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", text)
	}
	return value, nil
}

func (p *expressionParser) skipSpaces() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *expressionParser) match(ch byte) bool {
	if p.pos >= len(p.input) || p.input[p.pos] != ch {
		return false
	}
	p.pos++
	return true
}
