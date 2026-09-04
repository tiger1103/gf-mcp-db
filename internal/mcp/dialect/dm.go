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

type dmDialect struct {
	oracleLikeDialect
}
