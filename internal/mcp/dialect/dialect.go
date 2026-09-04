/*
 * @desc:数据库方言层：封装 gdb 元数据 API 之外的库差异
 * （标识符引用符、LIMIT 语法、索引/外键/表状态查询）
 *
 * 统一输出键约定：
 *   Indexes：index_name / column_name / is_unique
 *   ForeignKeys：constraint_name / column_name / referenced_table_name / referenced_column_name
 *   TableStat：rows_estimate / table_comment / engine / data_length / index_length（按库可得性，缺则省略键）
 */

package dialect

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/util/gconv"
)

// IDialect 数据库方言接口。
// 约定：Indexes/ForeignKeys/TableStat 返回统一键（见包注释）；
// 查询 SQL 一律使用 ? 占位符（各 contrib 驱动的 DoFilter 负责转换为 $n/@pN/:vN）。
type IDialect interface {
	// Name 方言名（与 gdb 驱动类型一致）
	Name() string
	// QuoteIdent 引用标识符（表名/列名），非法字符直接 panic（工具层在 g.Try 内调用）
	QuoteIdent(name string) string
	// Paginate 为 SELECT 语句施加条数限制
	Paginate(selectSQL string, limit int) string
	// Indexes 表索引列表
	Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error)
	// ForeignKeys 表外键列表
	ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error)
	// TableStat 表统计信息（行数估算/引擎/注释），无对应能力返回 (nil, nil)
	TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error)
	// PrimaryKeys 返回表的主键列名集合（列名为键）。默认实现基于 TableFields 的 Key 标记；
	// 元数据不回填 Key 的库（如 Oracle/DM）在各自方言中覆盖。
	PrimaryKeys(ctx context.Context, db gdb.DB, table string) (map[string]bool, error)
}

// 注册表
var dialects = map[string]IDialect{}

// Register 注册方言（仅允许在各方言文件的 init() 中调用；重复注册同名方言视为编码错误）
func Register(d IDialect) {
	name := strings.ToLower(d.Name())
	if _, exists := dialects[name]; exists {
		panic(gerror.NewCodef(gcode.CodeInvalidParameter, "方言 %q 重复注册", d.Name()))
	}
	dialects[name] = d
}

// Get 获取指定类型的方言
func Get(dbType string) (IDialect, error) {
	if d, ok := dialects[strings.ToLower(dbType)]; ok {
		return d, nil
	}
	return nil, gerror.NewCodef(gcode.CodeInvalidParameter, "数据库类型 %q 没有可用方言", dbType)
}

// identReg 合法标识符（支持 $、#，如 Oracle 派生名）
var identReg = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$#]*$`)

// BaseDialect 方言基类：默认 LIMIT 分页 + 按引用符引用标识符
type BaseDialect struct {
	name       string
	quoteLeft  string
	quoteRight string
}

// NewBaseDialect 构造基类
func NewBaseDialect(name, quoteLeft, quoteRight string) BaseDialect {
	return BaseDialect{name: name, quoteLeft: quoteLeft, quoteRight: quoteRight}
}

// Name 返回方言名
func (d *BaseDialect) Name() string { return d.name }

// QuoteIdent 引用标识符，支持 a.b 形式（逐段引用）；仅支持 ASCII 标识符（含表/列名为中文等非 ASCII 字符的库不适用，后续可扩展）
func (d *BaseDialect) QuoteIdent(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		panic(gerror.NewCode(gcode.CodeInvalidParameter, "标识符不能为空"))
	}
	parts := strings.Split(name, ".")
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		if !identReg.MatchString(p) {
			panic(gerror.NewCodef(gcode.CodeInvalidParameter, "非法标识符：%q", p))
		}
		quoted = append(quoted, d.quoteLeft+p+d.quoteRight)
	}
	return strings.Join(quoted, ".")
}

// Paginate 默认实现：LIMIT n（追加前去掉结尾的 ;）
func (d *BaseDialect) Paginate(selectSQL string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	sqlStr := strings.TrimRight(strings.TrimSpace(selectSQL), "; \t\n\r")
	return fmt.Sprintf("%s LIMIT %d", sqlStr, limit)
}

// PrimaryKeys 默认实现：从 TableFields 中取 Key 为 PRI 的列（mysql/pgsql/sqlite 等驱动回填该标记）
func (d *BaseDialect) PrimaryKeys(ctx context.Context, db gdb.DB, table string) (map[string]bool, error) {
	fields, err := db.TableFields(ctx, table)
	if err != nil {
		return nil, err
	}
	pks := make(map[string]bool)
	for _, f := range fields {
		if strings.EqualFold(f.Key, "PRI") {
			pks[f.Name] = true
		}
	}
	return pks, nil
}

// MatchPattern 判断表名是否匹配 SQL LIKE 风格模式（% 任意串、_ 单字符，大小写不敏感）
func MatchPattern(name, pattern string) bool {
	if strings.TrimSpace(pattern) == "" {
		return true
	}
	var sb strings.Builder
	for _, r := range pattern {
		switch r {
		case '%':
			sb.WriteString(".*")
		case '_':
			sb.WriteString(".")
		default:
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	re, err := regexp.Compile(`(?i)^` + sb.String() + `$`)
	if err != nil {
		return true
	}
	return re.MatchString(name)
}

// ColumnsFromTableFields 将 gdb.TableFields 转为统一列结构（按列序输出）。
// primaryKeys 为主键列名集合（可传 nil，则所有列 primary_key 为 false）。
func ColumnsFromTableFields(fields map[string]*gdb.TableField, primaryKeys map[string]bool) []map[string]string {
	type indexedCol struct {
		index int
		col   map[string]string
	}
	cols := make([]indexedCol, 0, len(fields))
	for _, f := range fields {
		cols = append(cols, indexedCol{
			index: f.Index,
			col: map[string]string{
				"column_name": f.Name,
				"data_type":   f.Type,
				"nullable":    strconv.FormatBool(f.Null),
				"primary_key": strconv.FormatBool(primaryKeys[f.Name]),
				"default":     gconv.String(f.Default),
				"extra":       f.Extra,
				"comment":     f.Comment,
			},
		})
	}
	sort.Slice(cols, func(i, j int) bool { return cols[i].index < cols[j].index })
	out := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, c.col)
	}
	return out
}
