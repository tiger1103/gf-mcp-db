/*
 * @desc:ClickHouse 方言（无传统二级索引/外键；标识符反引号；分页 LIMIT）
 *
 * 版本假设：ClickHouse 21.x+（system.columns.is_in_primary_key、
 * system.tables.comment、system.data_skipping_indices）。
 */

package dialect

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

// chTableColumn `table` 在 ClickHouse 中为关键字，system.columns/system.tables 查询需反引号引用
const chTableColumn = "`table`"

func init() {
	Register(NewClickhouseDialect())
}

// NewClickhouseDialect 构造 ClickHouse 方言
func NewClickhouseDialect() IDialect {
	return &clickhouseDialect{BaseDialect: NewBaseDialect("clickhouse", "`", "`")}
}

type clickhouseDialect struct {
	BaseDialect
}

// TableFields 覆盖基类：gf clickhouse 驱动的列查询未按当前库过滤且直接拼接表名，
// 这里改为按 currentDatabase() 限定、表名走占位符绑定，避免跨库同名表串列与注入风险。
func (d *clickhouseDialect) TableFields(ctx context.Context, db gdb.DB, table string) (map[string]*gdb.TableField, error) {
	res, err := db.Query(ctx, `
		SELECT name, position, type, default_expression, comment, is_in_primary_key
		FROM system.columns
		WHERE database = currentDatabase() AND `+chTableColumn+` = ?
		ORDER BY position`, table)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]*gdb.TableField, len(res))
	for _, r := range res {
		name := r["name"].String()
		key := ""
		if r["is_in_primary_key"].Bool() {
			key = "PRI"
		}
		fields[name] = &gdb.TableField{
			// system.columns.position 从 1 开始；Index 从 0 开始用于列序
			Index:   r["position"].Int() - 1,
			Name:    name,
			Type:    r["type"].String(),
			Null:    clickhouseTypeNullable(r["type"].String()),
			Key:     key,
			Default: r["default_expression"].Val(),
			Comment: r["comment"].String(),
		}
	}
	return fields, nil
}

// clickhouseTypeNullable 判断类型是否可空。ClickHouse 以 Nullable(T) 表达可空，
// 且可能被 LowCardinality 等外层包装，故剥离包装后判断。
func clickhouseTypeNullable(fieldType string) bool {
	for {
		switch {
		case strings.HasPrefix(fieldType, "Nullable(") && strings.HasSuffix(fieldType, ")"):
			return true
		case strings.HasPrefix(fieldType, "LowCardinality(") && strings.HasSuffix(fieldType, ")"):
			fieldType = fieldType[len("LowCardinality(") : len(fieldType)-1]
		default:
			return false
		}
	}
}

// PrimaryKeys 主键列集合（system.columns.is_in_primary_key；MergeTree 的 ORDER BY 即主键）
func (d *clickhouseDialect) PrimaryKeys(ctx context.Context, db gdb.DB, table string) (map[string]bool, error) {
	res, err := db.Query(ctx, `
		SELECT name
		FROM system.columns
		WHERE database = currentDatabase() AND `+chTableColumn+` = ? AND is_in_primary_key = 1
		ORDER BY position`, table)
	if err != nil {
		return nil, err
	}
	pks := make(map[string]bool, len(res))
	for _, r := range res {
		pks[r["name"].String()] = true
	}
	return pks, nil
}

// Indexes 索引列表。ClickHouse 无传统二级索引，返回数据跳过索引（system.data_skipping_indices）；
// column_name 为该索引的表达式（如 url、lower(url)），is_unique 恒为 false（ClickHouse 索引不保证唯一）。
func (d *clickhouseDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT name AS index_name, expr AS column_name
		FROM system.data_skipping_indices
		WHERE database = currentDatabase() AND `+chTableColumn+` = ?
		ORDER BY name`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name":  r["index_name"],
			"column_name": r["column_name"],
			"is_unique":   gvar.New(false),
		})
	}
	return out, nil
}

// ForeignKeys ClickHouse 不支持外键约束，恒返回空列表（工具层据此省略 foreign_keys 键）
func (d *clickhouseDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	return gdb.Result{}, nil
}

// TableStat 表统计（system.tables）：engine/注释/行数估算/数据体积。
// 非 MergeTree 引擎（如 Memory、View）的 total_rows 可能为 NULL，此时省略 rows_estimate；
// ClickHouse 无索引体积概念，不提供 index_length。
func (d *clickhouseDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT engine, total_rows, total_bytes, comment
		FROM system.tables
		WHERE database = currentDatabase() AND name = ?`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	r := res[0]
	stat := gdb.Record{
		"engine":        r["engine"],
		"table_comment": r["comment"],
		"data_length":   r["total_bytes"],
	}
	if !r["total_rows"].IsNil() {
		stat["rows_estimate"] = r["total_rows"]
	}
	return stat, nil
}
