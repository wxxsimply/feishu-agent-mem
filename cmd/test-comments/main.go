package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/test-comments/main.go <doc-token>")
	}
	docToken := os.Args[1]

	cmd := exec.Command("lark-cli", "drive", "file.comments", "list",
		"--params", fmt.Sprintf(`{"file_token":"%s","file_type":"docx"}`, docToken),
		"--as", "user",
		"--format", "json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("lark-cli failed: %v\nOutput: %s", err, string(output))
	}

	fmt.Println("Raw output:")
	fmt.Println(string(output))
	fmt.Println("\n===== Parsed =====")

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		log.Fatalf("Unmarshal failed: %v", err)
	}

	data, ok := result["data"].(map[string]any)
	if !ok {
		log.Fatalf("No data field or not an object")
	}

	items, ok := data["items"].([]any)
	if !ok {
		log.Println("No items array")
		return
	}

	fmt.Printf("Found %d comments\n\n", len(items))
	for i, item := range items {
		itemBytes, _ := json.MarshalIndent(item, "", "  ")
		fmt.Printf("Comment #%d:\n%s\n\n", i, string(itemBytes))
	}
}
