package tools

import (
	"context"
	"time"
)

// TimeTool 返回当前环境时间。
// now 以函数形式注入，便于测试使用固定时间而不依赖真实时钟。
type TimeTool struct {
	now func() time.Time
}

// NewTimeTool 创建使用真实系统时间的 time 工具。
func NewTimeTool() *TimeTool {
	return &TimeTool{now: time.Now}
}

// NewTimeToolWithClock 创建使用自定义时钟的 time 工具，主要用于测试。
func NewTimeToolWithClock(now func() time.Time) *TimeTool {
	return &TimeTool{now: now}
}

func (t *TimeTool) Name() string {
	return "time"
}

func (t *TimeTool) Description() string {
	return "返回当前日期、时间、时区和 Unix 时间戳"
}

func (t *TimeTool) Parameters() []Parameter {
	return nil
}

// Execute 返回稳定格式的时间文本和可审计 metadata。
func (t *TimeTool) Execute(_ context.Context, _ map[string]any) ToolResult {
	now := t.now()
	zoneName, zoneOffset := now.Zone()
	return successResult(now.Format(time.RFC3339), map[string]any{
		"date":                now.Format("2006-01-02"),
		"time":                now.Format("15:04:05"),
		"timezone":            zoneName,
		"timezone_offset_sec": zoneOffset,
		"unix":                now.Unix(),
	})
}
