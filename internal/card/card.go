package card

import (
	"encoding/json"
	"fmt"
	"time"

	"feishu-mem/internal/decision"
	"feishu-mem/internal/recall"
)

// Renderer 卡片渲染器
type Renderer struct {
}

// NewRenderer 创建卡片渲染器
func NewRenderer() *Renderer {
	return &Renderer{}
}

// RenderLarkCard 渲染飞书交互卡片
func (r *Renderer) RenderLarkCard(card *recall.DecisionCard) (string, error) {
	if card == nil || card.Decision == nil {
		return "", fmt.Errorf("invalid decision card")
	}

	node := card.Decision

	category := r.getHotCategory(card.HotScore)

	cardContent := larkCard{
		MsgType: "interactive",
		Card: larkCardContent{
			Header: larkCardHeader{
				Title: larkCardText{
					Tag:     "plain_text",
					Content: fmt.Sprintf("%s 决策卡片 [%s]", r.getCategoryIcon(category), node.SDRID),
				},
				Template: r.getStatusTemplate(node.Status),
			},
			Elements: r.buildCardElements(node, card.HotScore, category),
		},
	}

	data, err := json.Marshal(cardContent)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// RenderLarkCardFromNode 渲染单个决策为飞书卡片
func (r *Renderer) RenderLarkCardFromNode(node *decision.DecisionNode, hotScore float64) (string, error) {
	card := &recall.DecisionCard{
		Decision:    node,
		HotScore:    hotScore,
		RecallType: recall.RecallExact,
	}
	return r.RenderLarkCard(card)
}

// RenderDailySummary 渲染每日摘要卡片
func (r *Renderer) RenderDailySummary(
	date time.Time,
	newDecisions []*recall.DecisionCard,
	forgottenDecisions []*recall.DecisionCard,
) (string, error) {

	cardContent := larkCard{
		MsgType: "interactive",
		Card: larkCardContent{
			Header: larkCardHeader{
				Title: larkCardText{
					Tag:     "plain_text",
					Content: fmt.Sprintf("📋 决策日报 - %s", date.Format("2006-01-02")),
				},
				Template: "blue",
			},
			Elements: []interface{}{},
		},
	}

	// 新增决策部分
	if len(newDecisions) > 0 {
		cardContent.Card.Elements = append(cardContent.Card.Elements,
			larkCardDiv{
				Tag: "div",
				Text: larkCardText{
					Tag:     "lark_md",
					Content: fmt.Sprintf("**✨ 新增决策 (%d 个)**", len(newDecisions)),
				},
			},
		)

		for i, dc := range newDecisions {
			if i >= 5 { // 最多显示5个
				break
			}
			cardContent.Card.Elements = append(cardContent.Card.Elements,
				larkCardDiv{
					Tag: "div",
					Text: larkCardText{
						Tag:     "lark_md",
						Content: fmt.Sprintf("• **%s** [%s] - 🔥%.0f", dc.Decision.Title, dc.Decision.SDRID, dc.HotScore),
					},
				},
			)
		}
	}

	// 遗忘决策提醒
	if len(forgottenDecisions) > 0 {
		cardContent.Card.Elements = append(cardContent.Card.Elements,
			larkCardHr{Tag: "hr"},
			larkCardDiv{
				Tag: "div",
				Text: larkCardText{
					Tag:     "lark_md",
					Content: fmt.Sprintf("**💤 遗忘决策提醒 (%d 个)**", len(forgottenDecisions)),
				},
			},
		)

		for i, dc := range forgottenDecisions {
			if i >= 3 { // 最多显示3个
				break
			}
			cardContent.Card.Elements = append(cardContent.Card.Elements,
				larkCardDiv{
					Tag: "div",
					Text: larkCardText{
						Tag:     "lark_md",
						Content: fmt.Sprintf("• **%s** [%s] - 🔥%.0f", dc.Decision.Title, dc.Decision.SDRID, dc.HotScore),
					},
				},
			)
		}
	}

	data, err := json.Marshal(cardContent)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// 内部辅助方法

func (r *Renderer) buildCardElements(node *decision.DecisionNode, hotScore float64, category recall.HotCategory) []interface{} {
	var elements []interface{}

	// 基本信息字段
	elements = append(elements, larkCardDiv{
		Tag: "div",
		Fields: []larkCardField{
			{
				IsShort: true,
				Text: larkCardText{
					Tag:     "lark_md",
					Content: fmt.Sprintf("**📌 标题**\n%s", node.Title),
				},
			},
			{
				IsShort: true,
				Text: larkCardText{
					Tag:     "lark_md",
					Content: fmt.Sprintf("**📊 状态**\n%s | %s", node.Status, node.ImpactLevel),
				},
			},
			{
				IsShort: true,
				Text: larkCardText{
					Tag:     "lark_md",
					Content: fmt.Sprintf("**🏷️ 议题**\n%s", node.Topic),
				},
			},
			{
				IsShort: true,
				Text: larkCardText{
					Tag:     "lark_md",
					Content: fmt.Sprintf("**📅 创建时间**\n%s", node.CreatedAt.Format("2006-01-02 15:04")),
				},
			},
		},
	})

	// 决策内容
	if node.Decision != "" {
		elements = append(elements, larkCardDiv{
			Tag: "div",
			Text: larkCardText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**📝 决策内容**\n%s", node.Decision),
			},
		})
	}

	// 决策依据
	if node.Rationale != "" {
		elements = append(elements, larkCardDiv{
			Tag: "div",
			Text: larkCardText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**💡 决策依据**\n%s", node.Rationale),
			},
		})
	}

	// 人员信息
	if node.Proposer != "" || node.Executor != "" {
		var peopleInfo string
		if node.Proposer != "" {
			peopleInfo += fmt.Sprintf("**👤 提议者**: %s  ", node.Proposer)
		}
		if node.Executor != "" {
			peopleInfo += fmt.Sprintf("**👤 执行者**: %s", node.Executor)
		}
		elements = append(elements, larkCardDiv{
			Tag: "div",
			Text: larkCardText{
				Tag:     "lark_md",
				Content: peopleInfo,
			},
		})
	}

	// 分隔线
	elements = append(elements, larkCardHr{Tag: "hr"})

	// 操作按钮 - 构建按钮列表
	buttons := []larkCardButton{
		{
			Tag:  "button",
			Text: larkCardText{Tag: "plain_text", Content: "查看详情"},
			Type: "primary",
			Value: map[string]interface{}{"action": "view_detail", "sdr_id": node.SDRID},
		},
	}

	relatedCount := len(node.Relations)
	if relatedCount > 0 {
		buttons = append(buttons, larkCardButton{
			Tag:  "button",
			Text: larkCardText{Tag: "plain_text", Content: fmt.Sprintf("关联决策 (%d)", relatedCount)},
			Type: "default",
			Value: map[string]interface{}{"action": "view_related", "sdr_id": node.SDRID},
		})
	}

	buttons = append(buttons, larkCardButton{
		Tag:  "button",
		Text: larkCardText{Tag: "plain_text", Content: "更新状态"},
		Type: "default",
		Value: map[string]interface{}{"action": "update_status", "sdr_id": node.SDRID},
	})

	elements = append(elements, larkCardAction{
		Tag:     "action",
		Actions: buttons,
	})

	// 底部备注
	elements = append(elements, larkCardNote{
		Tag: "note",
		Elements: []larkCardText{
			{
				Tag:     "plain_text",
				Content: fmt.Sprintf("🔥 热点值: %.0f/100 (%s)", hotScore, category),
			},
		},
	})

	return elements
}

func (r *Renderer) getStatusTemplate(status decision.DecisionStatus) string {
	switch status {
	case decision.StatusCompleted:
		return "green"
	case decision.StatusInDiscussion:
		return "yellow"
	case decision.StatusRejected, decision.StatusDeprecated, decision.StatusShelved:
		return "red"
	default:
		return "blue"
	}
}

func (r *Renderer) getHotCategory(hotScore float64) recall.HotCategory {
	switch {
	case hotScore >= 80:
		return recall.HotCategoryActive
	case hotScore >= 50:
		return recall.HotCategoryNormal
	case hotScore >= 20:
		return recall.HotCategoryFuzzy
	default:
		return recall.HotCategoryForgotten
	}
}

func (r *Renderer) getCategoryIcon(category recall.HotCategory) string {
	switch category {
	case recall.HotCategoryActive:
		return "🔥"
	case recall.HotCategoryNormal:
		return "📊"
	case recall.HotCategoryFuzzy:
		return "💤"
	case recall.HotCategoryForgotten:
		return "🪦"
	default:
		return "📋"
	}
}

// 飞书卡片类型定义（小写，内部使用）

type larkCard struct {
	MsgType string         `json:"msg_type"`
	Card    larkCardContent `json:"card"`
}

type larkCardContent struct {
	Header  larkCardHeader `json:"header"`
	Elements []interface{} `json:"elements"`
}

type larkCardHeader struct {
	Title    larkCardText `json:"title"`
	Template string      `json:"template"`
}

type larkCardText struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

type larkCardDiv struct {
	Tag    string         `json:"tag"`
	Text   larkCardText  `json:"text,omitempty"`
	Fields []larkCardField `json:"fields,omitempty"`
}

type larkCardField struct {
	IsShort bool        `json:"is_short"`
	Text    larkCardText `json:"text"`
}

type larkCardHr struct {
	Tag string `json:"tag"`
}

type larkCardAction struct {
	Tag     string          `json:"tag"`
	Actions []larkCardButton `json:"actions"`
}

type larkCardButton struct {
	Tag   string                 `json:"tag"`
	Text  larkCardText           `json:"text"`
	Type  string                 `json:"type"`
	Value map[string]interface{} `json:"value"`
}

type larkCardNote struct {
	Tag      string         `json:"tag"`
	Elements []larkCardText `json:"elements"`
}
