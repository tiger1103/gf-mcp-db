/*
 * @desc:PostgreSQL 方言
 */

package dialect

import (
	"context"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewPgsqlDialect())
}

// NewPgsqlDialect 构造 PostgreSQL 方言
func NewPgsqlDialect() IDialect {
	return &pgsqlDialect{BaseDialect: NewBaseDialect("pgsql", `"`, `"`)}
}

type pgsqlDialect struct {
	BaseDialect
}

// Indexes 索引列表（pg_index / pg_attribute；表达式索引的列无法以普通列名表达，暂不列出）
func (d *pgsqlDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT i.relname AS index_name, a.attname AS column_name, ix.indisunique AS is_unique
		FROM pg_class t
		JOIN pg_index ix ON ix.indrelid = t.oid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE t.relname = ? AND n.nspname = current_schema()
		ORDER BY i.relname, a.attnum`, table)
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

// ForeignKeys 外键列表（pg_constraint：约束名仅表内唯一，information_schema 同名约束会跨表误配）。
// 注意：使用 unnest WITH ORDINALITY，需 PostgreSQL 9.4+。
func (d *pgsqlDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT con.conname AS constraint_name,
		       src_att.attname AS column_name,
		       tgt.relname AS referenced_table_name,
		       tgt_att.attname AS referenced_column_name
		FROM pg_constraint con
		JOIN pg_class src ON src.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = src.relnamespace
		JOIN pg_class tgt ON tgt.oid = con.confrelid
		CROSS JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS ck(attnum, ord)
		JOIN pg_attribute src_att ON src_att.attrelid = con.conrelid AND src_att.attnum = ck.attnum
		JOIN pg_attribute tgt_att ON tgt_att.attrelid = con.confrelid AND tgt_att.attnum = con.confkey[ck.ord]
		WHERE con.contype = 'f' AND src.relname = ? AND n.nspname = current_schema()
		ORDER BY con.conname, ck.ord`, table)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// TableStat 行数估算与表注释（pg_class，限当前 schema）；reltuples < 0 表示未 ANALYZE（PG 13+），省略该键。
// 注：PG 12 及以下未 ANALYZE 的表 reltuples 为 0，会显示 rows_estimate=0。
func (d *pgsqlDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT c.reltuples::bigint AS rows_estimate, obj_description(c.oid, 'pg_class') AS table_comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = ? AND c.relkind IN ('r','p') AND n.nspname = current_schema()`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	r := res[0]
	stat := gdb.Record{"table_comment": r["table_comment"]}
	if rows := r["rows_estimate"].Int(); rows >= 0 {
		stat["rows_estimate"] = gvar.New(rows)
	}
	return stat, nil
}
