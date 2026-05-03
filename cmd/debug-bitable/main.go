package main

import (
	"encoding/json"
	"fmt"
	"log"

	larkadapter "feishu-mem/internal/lark-adapter"
)

func main() {
	baseToken := "NnnMb5mWJaBJkXsHf69cIpfMn8b"
	tableID := "tblEBXkSxaqxnY6l"

	larkCLI := larkadapter.NewLarkCLI()

	fmt.Println("=== 调试 Bitable 数据结构 ===")
	fmt.Println("正在获取 Bitable 数据...")

	output, err := larkCLI.RunCommand(
		"base", "+record-list",
		"--base-token", baseToken,
		"--table-id", tableID,
	)
	if err != nil {
		log.Fatalf("获取数据失败: %v", err)
	}

	fmt.Printf("\n原始输出:\n%s\n", string(output))

	fmt.Println("\n--- 解析 JSON 数据 ---")
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		log.Printf("JSON 解析失败: %v", err)
	} else {
		fmt.Printf("顶级键: ")
		for k := range result {
			fmt.Printf("%s ", k)
		}
		fmt.Println()

		if items, ok := result["items"].([]interface{}); ok {
			fmt.Printf("有 %d 条记录\n", len(items))
			for i, item := range items {
				if rec, ok := item.(map[string]interface{}); ok {
					fmt.Printf("\n记录 %d:\n", i+1)
					fmt.Printf("  record_id: %v\n", rec["record_id"])
					if fields, ok := rec["fields"].(map[string]interface{}); ok {
						fmt.Printf("  字段列表:\n")
						for k, v := range fields {
							fmt.Printf("    %s: %v (%T)\n", k, v, v)
						}
					}
				}
			}
		}
	}

	fmt.Println("\n--- 调试完成 ---")
}
