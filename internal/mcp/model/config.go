/*
* @desc:配置文件model
* @company:云南奇讯科技有限公司
* @Author: yixiaohu<yxh669@qq.com>
* @Date:   2025/4/24 10:01
 */

package model

type DatabaseConfig struct {
	Default *ConfigNode `json:"default"`
	Logger  *struct {
		Path   string `json:"Path"`
		Level  string `json:"level"`
		Stdout bool   `json:"stdout"`
	} `json:"logger"`
}

type ConfigNode struct {
	Charset     string `json:"charset"`
	Debug       bool   `json:"debug"`
	DryRun      bool   `json:"dryRun"`
	Link        string `json:"link"`
	MaxIdle     int    `json:"maxIdle"`
	MaxLifetime string `json:"maxLifetime"`
	MaxOpen     int    `json:"maxOpen"`
}

type GenConfig struct {
	ApiName       string `json:"apiName"`
	Author        string `json:"author"`
	AutoRemovePre bool   `json:"autoRemovePre"`
	FrontDir      string `json:"frontDir"`
	GoModName     string `json:"goModName"`
	ModuleName    string `json:"moduleName"`
	PackageName   string `json:"packageName"`
	TablePrefix   string `json:"tablePrefix"`
	TemplatePath  string `json:"templatePath"`
}
type Config struct {
	Database *DatabaseConfig `json:"database"`
	Gen      *GenConfig      `json:"gen"`
}
