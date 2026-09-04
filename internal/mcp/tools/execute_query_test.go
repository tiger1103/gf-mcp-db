/*
 * @desc:execute_query 单元测试（纯函数部分）
 */

package tools

import "testing"

func TestIsQuerySQL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"SELECT * FROM t", true},
		{"  select 1", true},
		{"WITH c AS (SELECT 1) SELECT * FROM c", true},
		{"SHOW TABLES", true},
		{"EXPLAIN SELECT 1", true},
		{"DESC users", true},
		{"DESCRIBE users", true},
		{"PRAGMA table_info(users)", true},
		{"INSERT INTO t VALUES (1)", false},
		{"UPDATE t SET a=1", false},
		{"DELETE FROM t", false},
		{"CREATE TABLE t (id INT)", false},
		{"DROP TABLE t", false},
	}
	for _, c := range cases {
		if got := isQuerySQL(c.in); got != c.want {
			t.Fatalf("isQuerySQL(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
