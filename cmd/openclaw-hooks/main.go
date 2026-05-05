package main

import (
	"flag"
	"fmt"
	"os"

	"feishu-mem/internal/hooks"
)

func main() {
	var hookType string
	flag.StringVar(&hookType, "hook", "", "Hook type: pre-tool-use, post-tool-use")
	flag.Parse()

	if hookType != "" {
		switch hookType {
		case "pre-tool-use":
			hooks.RunPreToolUse()
			return
		case "post-tool-use":
			hooks.RunPostToolUse()
			return
		default:
			fmt.Printf("Unknown hook type: %s\n", hookType)
			os.Exit(1)
		}
	}

	var taskType string
	flag.StringVar(&taskType, "task", "", "Task type: sync, consistency, lark-detector")
	if flag.CommandLine.NFlag() > 0 {
		flag.Parse()
		if taskType != "" {
			hooks.RunScheduledTask()
			return
		}
	}

	fmt.Println("Usage:")
	fmt.Println("  As hook: --hook=pre-tool-use or --hook=post-tool-use")
	fmt.Println("  As task: --task=sync or --task=consistency or --task=lark-detector")
	os.Exit(1)
}
