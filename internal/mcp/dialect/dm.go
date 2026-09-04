/*
 * @desc:达梦（DM8）方言：复用 Oracle 兼容目录视图；分页用 DM8 支持的 LIMIT 语法
 */

package dialect

func init() {
	Register(NewDmDialect())
}

// NewDmDialect 构造 DM 方言
func NewDmDialect() IDialect {
	return &dmDialect{oracleLikeDialect{BaseDialect: NewBaseDialect("dm", `"`, `"`)}}
}

// dmDialect 达梦方言。
// 作用域假设：登录用户即目标模式（DM DSN 的 schema 通常与用户一致），
// 目录查询统一以 OWNER = USER 过滤；如需以 SYSDBA 等跨模式检查，结果为空。
type dmDialect struct {
	oracleLikeDialect
}
