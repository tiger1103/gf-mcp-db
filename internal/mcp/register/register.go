/*
 * @desc:MCP 注册器
 * @company:云南奇讯科技有限公司
 * @Author: yixiaohu<yxh669@qq.com>
 * @Date:   2025/4/17 21:56
 */

package register

import (
	"reflect"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

type Handler func(mcpServer *server.MCPServer)
type Register struct{}

var funcOptions = make([]Handler, 0)

func AddHandler(handler Handler) {
	funcOptions = append(funcOptions, handler)
}

func DoHandler(mcpServer *server.MCPServer) {
	for _, mcpTool := range funcOptions {
		mcpTool(mcpServer)
	}
}

func DoRegister(reg interface{}) {
	//TypeOf 会返回目标数据的类型，比如 int/float/struct/指针等
	typ := reflect.TypeOf(reg)
	//ValueOf 返回目标数据的的值
	val := reflect.ValueOf(reg)
	for i := 0; i < typ.NumMethod(); i++ {
		//调用绑定方法
		// 检查方法名是否以 "Register" 开头
		methodName := typ.Method(i).Name
		if strings.HasPrefix(methodName, "Register") {
			val.Method(i).Call(nil)
		}
	}
}
