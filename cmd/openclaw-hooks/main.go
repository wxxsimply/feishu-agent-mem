package main

import (
	"flag"
	"fmt"
	"os"

	"feishu-mem/internal/hooks"
)

func main() {
	// Define flags
	hookType := flag.String("hook", "", "Hook type: pre-tool-use, post-tool-use")
	taskType := flag.String("task", "", "Task type: sync, consistency, lark-detector")

	// Parse flags
	flag.Parse()

	// Check if hook mode
	if *hookType != "" {
		switch *hookType {
		case "pre-tool-use":
			hooks.RunPreToolUse()
			return
		case "post-tool-use":
			hooks.RunPostToolUse()
			return
		default:
			fmt.Printf("Unknown hook type: %s\n", *hookType)
			os.Exit(1)
		}
	}

	// Check if task mode
	if *taskType != "" {
		hooks.RunScheduledTaskWithType(*taskType)
		return
	}

	// Print usage
	fmt.Println("Usage:")
	fmt.Println("  As hook:   --hook=pre-tool-use  or  --hook=post-tool-use")
	fmt.Println("  As task:   --task=sync  or  --task=consistency  or  --task=lark-detector")
	os.Exit(1)
}
