package main

import (
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/detector"
)

const (
	version = "2.0.0"
)

func main() {
	docDetector := larkadapter.NewDocExtractor(larkadapter.LoadConfig())
	base := detector.NewBaseDetector("lark_doc", version, docDetector)

	if err := base.Initialize(); err != nil {
		panic(err)
	}

	base.Run()
}
