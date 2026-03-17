/*
* @desc:xxxx功能描述
* @company:云南奇讯科技有限公司
* @Author: yixiaohu<yxh669@qq.com>
* @Date:   2025/4/24 10:37
 */

package libUtils

import (
	"github.com/gogf/gf/v2/os/gcache"
	"sync"
)

var (
	cache *gcache.Cache
	lock  sync.Mutex
)

func Cache() *gcache.Cache {
	lock.Lock()
	defer lock.Unlock()
	if cache == nil {
		cache = gcache.New()
	}
	return cache
}
