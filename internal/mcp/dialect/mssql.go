/*
 * @desc:SQL Server 方言
 */

package dialect

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewMssqlDialect())
}

// NewMssqlDialect 构造 SQL Server 方言（引用符与 gf mssql 驱动一致，为双引号）
func NewMssqlDialect() IDialect {
	return &mssqlDialect{BaseDialect: NewBaseDialect("mssql", `"`, `"`)}
}

type mssqlDialect struct {
	BaseDialect
}

// Paginate TOP 语法（兼容 SELECT DISTINCT；先去除尾部分号/空白再改写，与基类 LIMIT 分支语义一致）；仅适配工具层构造的简单查询（SELECT [DISTINCT] col FROM t [WHERE][ORDER BY]），含 UNION/INTERSECT/EXCEPT/OFFSET 或已有 TOP 的语句不做改写保证
func (d *mssqlDialect) Paginate(selectSQL string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	s := strings.TrimRight(strings.TrimSpace(selectSQL), "; \t\n\r")
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "SELECT DISTINCT") {
		return "SELECT DISTINCT TOP " + strconv.Itoa(limit) + s[len("SELECT DISTINCT"):]
	}
	if strings.HasPrefix(upper, "SELECT") {
		return "SELECT TOP " + strconv.Itoa(limit) + s[len("SELECT"):]
	}
	return s
}

// Indexes 索引列表（sys.indexes / sys.index_columns）
func (d *mssqlDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT i.name AS index_name, c.name AS column_name, i.is_unique AS is_unique
		FROM sys.indexes i
		JOIN sys.tables t ON t.object_id = i.object_id
		JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
		JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
		WHERE t.name = ? AND t.schema_id = SCHEMA_ID() AND i.name IS NOT NULL
		ORDER BY i.name, ic.is_included_column, ic.key_ordinal, ic.index_column_id`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name":  r["index_name"],
			"column_name": r["column_name"],
			"is_unique":   gvar.New(r["is_unique"].Bool()),
		})
	}
	return out, nil
}

// ForeignKeys 外键列表（sys.foreign_keys）
func (d *mssqlDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT fk.name AS constraint_name, cp.name AS column_name,
		       tr.name AS referenced_table_name, cr.name AS referenced_column_name
		FROM sys.foreign_keys fk
		JOIN sys.tables tp ON tp.object_id = fk.parent_object_id
		JOIN sys.foreign_key_columns fkc ON fkc.constraint_object_id = fk.object_id
		JOIN sys.columns cp ON cp.object_id = fkc.parent_object_id AND cp.column_id = fkc.parent_column_id
		JOIN sys.tables tr ON tr.object_id = fkc.referenced_object_id
		JOIN sys.columns cr ON cr.object_id = fkc.referenced_object_id AND cr.column_id = fkc.referenced_column_id
		WHERE tp.name = ? AND tp.schema_id = SCHEMA_ID()
		ORDER BY fk.name, fkc.constraint_column_id`, table)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// TableStat 行数估算（sys.partitions；SQL Server 无便捷的表注释来源，故不提供）
func (d *mssqlDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT SUM(p.rows) AS rows_estimate
		FROM sys.partitions p
		JOIN sys.tables t ON t.object_id = p.object_id
		WHERE t.name = ? AND t.schema_id = SCHEMA_ID() AND p.index_id IN (0,1)`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	return gdb.Record{"rows_estimate": res[0]["rows_estimate"]}, nil
}
