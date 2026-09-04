/*
 * @desc:dbconn 单元测试
 */

package dbconn_test

import (
	"testing"

	"github.com/tiger1103/gf-mcp-db/internal/dbconn"
)

func TestNormalizeType(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"mysql", "mysql", true},
		{"MySQL", "mysql", true},
		{"postgres", "pgsql", true},
		{"postgresql", "pgsql", true},
		{"pgsql", "pgsql", true},
		{"sqlite", "sqlite", true},
		{"sqlite3", "sqlite", true},
		{"sqlserver", "mssql", true},
		{"mssql", "mssql", true},
		{"oracle", "oracle", true},
		{"dm", "dm", true},
		{"", "", false},
		{"foo", "", false},
	}
	for _, c := range cases {
		got, ok := dbconn.NormalizeType(c.in)
		if got != c.want || ok != c.wantOK {
			t.Fatalf("NormalizeType(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestBuildConfigNode(t *testing.T) {
	t.Run("sqlite 缺少文件路径报错", func(t *testing.T) {
		_, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "sqlite"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("mysql 缺少 host 报错", func(t *testing.T) {
		_, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "mysql", Username: "root", Database: "test"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("不支持类型报错", func(t *testing.T) {
		_, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "oracle19c"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("extra 格式非法报错", func(t *testing.T) {
		_, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "mysql", Host: "h", Username: "u", Database: "d", Extra: "not-a-kv"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("mysql 完整配置与默认值", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "mysql", Host: "127.0.0.1", Username: "root", Password: "p", Database: "test",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "mysql" || node.Host != "127.0.0.1" || node.Port != "3306" {
			t.Fatalf("字段不符: %+v", node)
		}
		if node.Charset != "utf8mb4" {
			t.Fatalf("mysql 默认字符集应为 utf8mb4: %q", node.Charset)
		}
		if node.User != "root" || node.Name != "test" {
			t.Fatalf("用户/库名不符: %+v", node)
		}
	})

	t.Run("postgres 别名与默认端口", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "postgres", Host: "192.168.0.214", Username: "postgres", Database: "test",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "pgsql" || node.Port != "5432" {
			t.Fatalf("pgsql 归一/端口不符: %+v", node)
		}
	})

	t.Run("oracle 默认端口", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "oracle", Host: "h", Username: "u", Password: "p", Database: "ORCL",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "oracle" || node.Port != "1521" || node.Name != "ORCL" {
			t.Fatalf("oracle 字段不符: %+v", node)
		}
	})

	t.Run("dm 默认端口与字符集", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "dm", Host: "h", Username: "u", Password: "p", Database: "DMSERVER",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "dm" || node.Port != "5236" || node.Charset != "UTF-8" {
			t.Fatalf("dm 字段不符: %+v", node)
		}
	})

	t.Run("mssql 默认端口", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "sqlserver", Host: "h", Username: "sa", Password: "p", Database: "master",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "mssql" || node.Port != "1433" {
			t.Fatalf("mssql 字段不符: %+v", node)
		}
	})

	t.Run("sqlite 合法配置（无端口要求）", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "sqlite", Database: "/tmp/a.db"})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "sqlite" || node.Name != "/tmp/a.db" || node.Port != "" {
			t.Fatalf("sqlite 字段不符: %+v", node)
		}
	})

	t.Run("extra 合法透传", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "mysql", Host: "h", Username: "u", Database: "d", Extra: "loc=Local",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Extra != "loc=Local" {
			t.Fatalf("extra 应透传: %+v", node)
		}
	})
}
