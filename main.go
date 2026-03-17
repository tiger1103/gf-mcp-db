package main

import (
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/os/gctx"

	_ "gf-mcp/internal/boot"
	"gf-mcp/internal/cmd"
	_ "gf-mcp/internal/packed"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
