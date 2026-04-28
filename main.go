package main

import (
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/tiger1103/gf-mcp-db/internal/boot"
	"github.com/tiger1103/gf-mcp-db/internal/cmd"
	_ "github.com/tiger1103/gf-mcp-db/internal/packed"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
