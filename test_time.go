package main

import (
	"fmt"
	"time"
)

func main() {
	// outputs/detect_state.json 里的 last_check
	lastCheckStr := "2026-05-06T14:40:31.257658+08:00"
	lastCheck, _ := time.Parse(time.RFC3339Nano, lastCheckStr)
	fmt.Printf("lastCheck: %v (Unix: %d)\n", lastCheck, lastCheck.Unix())

	// 文档的 update_time 从 +search 输出
	docUpdateUnix := int64(1778049493)
	docUpdateTime := time.Unix(docUpdateUnix, 0)
	fmt.Printf("Doc update_time: %v (Unix: %d)\n", docUpdateTime, docUpdateUnix)

	// 比较一下
	fmt.Printf("docUpdateTime < lastCheck? %v\n", docUpdateTime.Before(lastCheck))
	fmt.Printf("docUpdateUnix < lastCheck.Unix()? %v\n", docUpdateUnix < lastCheck.Unix())

	// 现在时间
	now := time.Now()
	fmt.Printf("Now: %v (Unix: %d)\n", now, now.Unix())
}
