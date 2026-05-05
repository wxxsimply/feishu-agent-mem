package git

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"time"

	"feishu-mem/internal/decision"
	"gopkg.in/yaml.v3"
)

// RenderDecisionFile 渲染决策文件为 YAML frontmatter + Markdown
func RenderDecisionFile(node *decision.DecisionNode) string {
	var buf bytes.Buffer

	// YAML frontmatter
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(node)
	buf.WriteString("---\n\n")

	// Markdown 正文
	buf.WriteString(fmt.Sprintf("# %s\n\n", node.Title))
	buf.WriteString("## 决策\n\n")
	buf.WriteString(node.Decision + "\n\n")
	if node.Rationale != "" {
		buf.WriteString("## 依据\n\n")
		buf.WriteString(node.Rationale + "\n\n")
	}

	// 元数据
	buf.WriteString("## 元数据\n\n")
	buf.WriteString(fmt.Sprintf("- **议题**: %s\n", node.Topic))
	buf.WriteString(fmt.Sprintf("- **状态**: %s\n", node.Status))
	buf.WriteString(fmt.Sprintf("- **影响级别**: %s\n", node.ImpactLevel))
	if node.Proposer != "" {
		buf.WriteString(fmt.Sprintf("- **提出人**: %s\n", node.Proposer))
	}
	if node.Executor != "" {
		buf.WriteString(fmt.Sprintf("- **执行人**: %s\n", node.Executor))
	}
	if node.DecisionType != "" {
		buf.WriteString(fmt.Sprintf("- **决策类型**: %s\n", node.DecisionType))
	}

	// 时间关联
	if node.ProjectPhase != "" || node.DecisionTime != "" || node.EffectiveTime != "" || node.Deadline != "" {
		buf.WriteString("\n## 时间关联\n\n")
		if node.ProjectPhase != "" {
			buf.WriteString(fmt.Sprintf("- **项目阶段**: %s\n", node.ProjectPhase))
		}
		if node.DecisionTime != "" {
			buf.WriteString(fmt.Sprintf("- **决策时间**: %s\n", node.DecisionTime))
		}
		if node.EffectiveTime != "" {
			buf.WriteString(fmt.Sprintf("- **生效时间**: %s\n", node.EffectiveTime))
		}
		if node.Deadline != "" {
			buf.WriteString(fmt.Sprintf("- **截止时间**: %s\n", node.Deadline))
		}
	}

	return buf.String()
}

// ParseDecisionFile 解析决策文件
func ParseDecisionFile(data []byte) (*decision.DecisionNode, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// 读取 YAML frontmatter
	var frontmatter []byte
	inFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
			} else {
				break
			}
		} else if inFrontmatter {
			frontmatter = append(frontmatter, line...)
			frontmatter = append(frontmatter, '\n')
		}
	}

	if len(frontmatter) == 0 {
		return nil, fmt.Errorf("no frontmatter found")
	}

	var node decision.DecisionNode
	if err := yaml.Unmarshal(frontmatter, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

// ParseObjectionFile 解析反对意见文件
func ParseObjectionFile(data []byte) (*decision.Objection, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var frontmatter bytes.Buffer
	inFrontmatter := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				break
			}
		} else if inFrontmatter {
			frontmatter.WriteString(line + "\n")
		}
	}
	if frontmatter.Len() == 0 {
		return nil, fmt.Errorf("no frontmatter found")
	}
	var obj decision.Objection
	if err := yaml.Unmarshal(frontmatter.Bytes(), &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

// FormatDecisionNode 格式化决策节点为简洁视图
func FormatDecisionNode(node *decision.DecisionNode) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("[%s] %s\n", node.SDRID, node.Title))
	buf.WriteString(fmt.Sprintf("  Topic: %s | Status: %s | Impact: %s\n",
		node.Topic, node.Status, node.ImpactLevel))
	if node.DecidedAt != nil {
		buf.WriteString(fmt.Sprintf("  Decided: %s\n", node.DecidedAt.Format(time.RFC3339)))
	}
	buf.WriteString(fmt.Sprintf("  Commit: %s\n", truncate(node.GitCommitHash, 8)))

	return buf.String()
}

// RenderObjectionFile 渲染反对意见文件为 YAML frontmatter + Markdown
func RenderObjectionFile(obj *decision.Objection) string {
	var buf bytes.Buffer

	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(obj)
	buf.WriteString("---\n\n")

	buf.WriteString(fmt.Sprintf("# Objection: %s\n\n", obj.ObjectionContent))
	if obj.Rationale != "" {
		buf.WriteString(fmt.Sprintf("## 理由\n\n%s\n\n", obj.Rationale))
	}
	if obj.Alternative != "" {
		buf.WriteString(fmt.Sprintf("## 替代方案\n\n%s\n\n", obj.Alternative))
	}
	buf.WriteString(fmt.Sprintf("- **反对人**: %s\n", obj.Objector))
	buf.WriteString(fmt.Sprintf("- **状态**: %s\n", obj.Status))
	if obj.ReferencesDecision != "" {
		buf.WriteString(fmt.Sprintf("- **关联决策**: %s\n", obj.ReferencesDecision))
	}
	buf.WriteString(fmt.Sprintf("- **来源**: %s\n", obj.SourceType))

	return buf.String()
}
