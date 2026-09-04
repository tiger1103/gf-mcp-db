/*
 * @desc:命令行入口（stdio / HTTP 模式选择）
 */

package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/dbconn"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/router"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/tools"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start universal database mcp server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// 检查是否应该使用 stdio 模式
			if isStdioMode() {
				return runStdioMode(ctx)
			}

			// HTTP SSE 模式
			return runHttpMode(ctx, parser)
		},
	}
)

// runStdioMode 运行 stdio 模式
func runStdioMode(ctx context.Context) error {
	g.Log().SetFlags(glog.F_ASYNC | glog.F_TIME_DATE | glog.F_TIME_TIME | glog.F_FILE_LONG)
	g.Log().Info(ctx, "Universal Database MCP server starting with stdio mode")

	// 从命令行参数解析数据库配置
	dbConfig := parseDbConfigFromArgs(os.Args[1:])

	// 如果提供了数据库配置，则初始化连接
	if dbConfig != nil && dbConfig.DBType != "" {
		g.Log().Info(ctx, "初始化数据库连接，类型："+dbConfig.DBType+", 数据库："+dbConfig.Database)
		if err := dbconn.Init(ctx, dbConfig); err != nil {
			g.Log().Error(ctx, "初始化数据库连接失败:", err)
			return err
		}
	} else {
		g.Log().Warning(ctx, "未提供数据库配置，将在工具调用时处理")
	}

	// 创建 MCPServer
	mcpServer := server.NewMCPServer("universal-db-mcp", "1.0.0")

	// 工具注册
	register.DoRegister(&tools.Reg{})
	register.DoHandler(mcpServer)

	// 创建 StdioServer 并启动
	stdioServer := server.NewStdioServer(mcpServer)
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

// runHttpMode 运行 HTTP SSE 模式（路由与连接初始化统一由 router 包完成）
func runHttpMode(ctx context.Context, parser *gcmd.Parser) error {
	g.Log().SetFlags(glog.F_ASYNC | glog.F_TIME_DATE | glog.F_TIME_TIME | glog.F_FILE_LONG)
	g.Log().Info(ctx, "Universal Database MCP server for sse starting")
	s := g.Server()
	router.Register(ctx, s)
	s.Run()
	return nil
}

// isStdioMode 检查是否应该使用 stdio 模式
func isStdioMode() bool {
	// 检查是否有 --stdio 标志
	for _, arg := range os.Args {
		if arg == "--stdio" || arg == "-s" {
			return true
		}
	}
	// 检查是否有数据库连接参数（stdio 模式特征）
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--type=") || strings.HasPrefix(arg, "--host=") ||
			strings.HasPrefix(arg, "--database=") || arg == "--type" || arg == "--host" || arg == "--database" {
			return true
		}
	}
	return false
}

// parseDbConfigFromArgs 从命令行参数解析数据库配置
func parseDbConfigFromArgs(args []string) *dbconn.Config {
	config := &dbconn.Config{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--type":
			if i+1 < len(args) {
				config.DBType = args[i+1]
				i++
			}
		case "--host":
			if i+1 < len(args) {
				config.Host = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				config.Port = args[i+1]
				i++
			}
		case "--user":
			if i+1 < len(args) {
				config.Username = args[i+1]
				i++
			}
		case "--password":
			if i+1 < len(args) {
				config.Password = args[i+1]
				i++
			}
		case "--database":
			if i+1 < len(args) {
				config.Database = args[i+1]
				i++
			}
		case "--charset":
			if i+1 < len(args) {
				config.Charset = args[i+1]
				i++
			}
		case "--extra":
			if i+1 < len(args) {
				config.Extra = args[i+1]
				i++
			}
		case "--debug":
			config.Debug = true
		}
	}
	return config
}

// 确保在包初始化时设置上下文
func init() {
	_ = gctx.GetInitCtx()
}
