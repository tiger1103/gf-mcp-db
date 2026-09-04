/*
 * @desc:MCP 路由注册（HTTP SSE 模式）
 */

package router

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/dbconn"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/tools"
)

// Register 注册 MCP 服务（HTTP SSE 模式）
func Register(ctx context.Context, s *ghttp.Server) {
	mcpServer := server.NewMCPServer("universal-db-mcp", "1.0.0")

	register.DoRegister(new(tools.Reg))
	register.DoHandler(mcpServer)

	sseServer := server.NewSSEServer(mcpServer,
		server.WithSSEEndpoint("/mcp"),
	)
	ssePath := sseServer.CompleteSsePath()
	messagePath := sseServer.CompleteMessagePath()

	s.BindHandler(ssePath, func(r *ghttp.Request) {
		initDBForRequest(ctx, r)
		sseServer.ServeHTTP(r.Response.Writer, r.Request)
	})
	s.BindHandler(messagePath, func(r *ghttp.Request) {
		initDBForRequest(ctx, r)
		sseServer.ServeHTTP(r.Response.Writer, r.Request)
	})
}

// initDBForRequest 从 query/header 解析数据库配置并初始化连接
func initDBForRequest(ctx context.Context, r *ghttp.Request) {
	config := parseDatabaseConfigFromQuery(r)
	if config.DBType == "" {
		config = parseDatabaseConfigFromHeader(r)
	}
	if config.DBType == "" {
		g.Log().Debug(ctx, "未提供数据库配置，跳过数据库连接初始化")
		return
	}
	if err := dbconn.Init(ctx, config); err != nil {
		glog.Error(ctx, "初始化数据库连接失败:", err)
		return
	}
	glog.Info(ctx, "数据库连接初始化成功，类型："+config.DBType+", 数据库："+config.Database+", SessionID:"+r.GetSessionId())
}

// parseDatabaseConfigFromQuery 从 URL 查询参数解析数据库配置
func parseDatabaseConfigFromQuery(r *ghttp.Request) *dbconn.Config {
	return &dbconn.Config{
		DBType:   r.Get("type").String(),
		Host:     r.Get("host").String(),
		Port:     r.Get("port").String(),
		Username: r.Get("user").String(),
		Password: r.Get("password").String(),
		Database: r.Get("database").String(),
		Charset:  r.Get("charset").String(),
		Extra:    r.Get("extra").String(),
		Debug:    r.Get("debug", "false").Bool(),
	}
}

// parseDatabaseConfigFromHeader 从请求头解析数据库配置
func parseDatabaseConfigFromHeader(r *ghttp.Request) *dbconn.Config {
	return &dbconn.Config{
		DBType:   r.GetHeader("X-DB-Type"),
		Host:     r.GetHeader("X-DB-Host"),
		Port:     r.GetHeader("X-DB-Port"),
		Username: r.GetHeader("X-DB-User"),
		Password: r.GetHeader("X-DB-Password"),
		Database: r.GetHeader("X-DB-Database"),
		Charset:  r.GetHeader("X-DB-Charset"),
		Extra:    r.GetHeader("X-DB-Extra"),
		Debug:    r.GetHeader("X-DB-Debug") == "true",
	}
}
