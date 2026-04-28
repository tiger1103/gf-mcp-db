/*
 * @desc:工具 tool
 * @company:云南奇讯科技有限公司
 * @Author: yixiaohu<yxh669@qq.com>
 * @Date:   2025/4/16 16:49
 */

package tools

import (
	"gf-mcp-db/internal/consts"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/mark3labs/mcp-go/mcp"
)

// Reg 工具注册结构
type Reg struct{}

// returnRes 统一返回处理
func (r *Reg) returnRes(err error, msgArr ...string) (*mcp.CallToolResult, error) {
	msg := "操作失败，原因："
	if len(msgArr) > 0 {
		msg = msgArr[0]
	}
	if v, ok := err.(*gerror.Error); ok {
		if v.Code().Code() != consts.CodeError {
			return mcp.NewToolResultText(msg + err.Error()), nil
		} else {
			return nil, err
		}
	}
	return nil, err
}
