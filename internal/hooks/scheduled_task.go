package hooks

import (
	"context"
	"fmt"
	"os"
	"time"

	"feishu-mem/internal/config"
	"feishu-mem/internal/core"
	"feishu-mem/internal/storage/git"
)

// RunScheduledTask (deprecated - use RunScheduledTaskWithType)
func RunScheduledTask() {
	// Keep for compatibility
	fmt.Println("RunScheduledTask deprecated")
	os.Exit(1)
}

type TaskType string

const (
	TaskSync         TaskType = "sync"
	TaskConsistency  TaskType = "consistency"
	TaskLarkDetector TaskType = "lark-detector"
)

func RunScheduledTaskWithType(taskType string) {
	fmt.Printf("Running task: %s\n", taskType)

	switch TaskType(taskType) {
	case TaskSync:
		runSyncTask()
	case TaskConsistency:
		runConsistencyCheck()
	case TaskLarkDetector:
		runLarkDetector()
	default:
		fmt.Printf("Unknown task type: %s\n", taskType)
		os.Exit(1)
	}
}

func runSyncTask() {
	fmt.Println("Running sync task (Git <-> Bitable)...")

	settings := config.DefaultSettings()
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		if s, err := config.LoadSettings(cfgPath); err == nil {
			settings = s
		}
	} else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
		settings = s
	}

	fmt.Printf("Using project: %s\n", settings.Project.Name)
	fmt.Println("⚠️  Full sync requires mem-service running with full pipeline")
	fmt.Println("For now, this is a placeholder - sync is handled by main pipeline")
	fmt.Println("✅ Sync task completed (standalone mode)")
}

func runConsistencyCheck() {
	fmt.Println("Running consistency check...")

	settings := config.DefaultSettings()
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		if s, err := config.LoadSettings(cfgPath); err == nil {
			settings = s
		}
	} else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
		settings = s
	}

	gitStorage, err := git.NewGitStorage(git.Config{
		WorkDir: settings.Git.WorkDir,
		Branch:  settings.Git.Branch,
	})

	if err != nil {
		fmt.Printf("Error initializing Git storage: %v\n", err)
		return
	}

	memoryGraph := core.NewMemoryGraph()
	if err := memoryGraph.LoadFromGit(gitStorage, settings.Project.Name); err != nil {
		fmt.Printf("Error loading from Git: %v\n", err)
	}

	decisions := memoryGraph.GetAllDecisions()
	fmt.Printf("Total decisions in Git: %d\n", len(decisions))

	// TODO: Bitable一致性检查
	fmt.Println("Consistency check completed (placeholder implementation)")
}

func runLarkDetector() {
	fmt.Println("Running lark detector...")
	// 这部分可以复用现有的检测器逻辑
	fmt.Println("Lark detector task completed (placeholder implementation)")
}

func runDetectorLoop(ctx context.Context, interval time.Duration) {
	fmt.Printf("Running detector loop with interval: %v\n", interval)
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context done, exiting detector loop")
			return
		case <-time.After(interval):
			fmt.Println("Detector tick")
			// 运行检测逻辑
		}
	}
}
