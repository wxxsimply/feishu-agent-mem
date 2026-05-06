package main

import (
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/detector"
)

const (
	version = "2.0.0"
)

func main() {
	imDetector := larkadapter.NewIMExtractor(larkadapter.LoadConfig())
	base := detector.NewBaseDetector("lark_im", version, imDetector)
	base.EnableLongConn() // 启用长连接模式

	if err := base.Initialize(); err != nil {
		panic(err)
	}

	base.Run()
}
