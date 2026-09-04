/*
 * @desc:helpers 单元测试（纯函数部分）
 */

package tools

import "testing"

func TestArgInt(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		key  string
		def  int
		want int
	}{
		{"缺失用默认", map[string]any{}, "limit", 10, 10},
		{"nil 用默认", map[string]any{"limit": nil}, "limit", 10, 10},
		{"int 正常", map[string]any{"limit": 5}, "limit", 10, 5},
		{"float64 正常（MCP JSON 数值）", map[string]any{"limit": float64(7)}, "limit", 10, 7},
		{"数字字符串", map[string]any{"limit": "3"}, "limit", 10, 3},
		{"布尔回落默认", map[string]any{"limit": true}, "limit", 10, 10},
		{"零用默认", map[string]any{"limit": 0}, "limit", 10, 10},
		{"负数用默认", map[string]any{"limit": -2}, "limit", 10, 10},
		{"垃圾值用默认", map[string]any{"limit": map[string]any{"a": 1}}, "limit", 10, 10},
	}
	for _, c := range cases {
		if got := argInt(c.args, c.key, c.def); got != c.want {
			t.Fatalf("%s: argInt = %d, want %d", c.name, got, c.want)
		}
	}
}
