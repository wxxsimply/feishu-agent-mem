package main

import (
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/detector"
)

const (
	version = "2.0.0"
)

func main() {
	calendarDetector := larkadapter.NewCalendarExtractor(larkadapter.LoadConfig())
	base := detector.NewBaseDetector("lark_calendar", version, calendarDetector)

	if err := base.Initialize(); err != nil {
		panic(err)
	}

	base.Run()
}
