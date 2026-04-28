package main

import (
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/os/gctx"

	_ "gf-mcp-db/internal/boot"
	"gf-mcp-db/internal/cmd"
	_ "gf-mcp-db/internal/packed"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
