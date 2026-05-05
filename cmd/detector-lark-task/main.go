package main

import (
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/detector"
)

const (
	version = "2.0.0"
)

func main() {
	taskDetector := larkadapter.NewTaskExtractor(larkadapter.LoadConfig())
	base := detector.NewBaseDetector("lark_task", version, taskDetector)

	if err := base.Initialize(); err != nil {
		panic(err)
	}

	base.Run()
}
