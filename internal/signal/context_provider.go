package signal

import (
	"fmt"
	"strings"

	"feishu-mem/internal/decision"
)

// ContextProvider 上下文提供者接口
type ContextProvider interface {
	// Name 提供者名称
	Name() string
	// GetRelevantContext 获取相关上下文（返回摘要字符串列表）
	GetRelevantContext(sig *StateChangeSignal, allDecisions []*decision.DecisionNode) ([]string, error)
}

// ===== IM 上下文提供者 =====

// IMContextProvider 提供同 Chat 的历史对话决策上下文
type IMContextProvider struct {
}

func NewIMContextProvider() *IMContextProvider {
	return &IMContextProvider{}
}

func (p *IMContextProvider) Name() string {
	return "im"
}

func (p *IMContextProvider) GetRelevantContext(
	sig *StateChangeSignal, allDecisions []*decision.DecisionNode,
) ([]string, error) {
	var summaries []string

	// 从信号的 PrimaryID 或 RelatedIDs 中提取 Chat 相关标识
	searchKeywords := p.extractKeywords(sig)

	// 1. 第一优先级：通过 ContentSnippet 匹配相关决策
	for _, d := range allDecisions {
		if len(summaries) >= 5 { // 最多 5 个
			break
		}
		if p.isRelevantIMDecision(d, searchKeywords, sig) {
			summary := fmt.Sprintf(
				"[Chat][%s] %s: %s",
				d.Status, d.Title, d.Decision)
			summaries = append(summaries, summary)
		}
	}

	// 2. 第二优先级：其他有 RelatedChatIDs 的决策
	if len(summaries) < 5 {
		for _, d := range allDecisions {
			if len(summaries) >= 5 {
				break
			}
			if len(d.FeishuLinks.RelatedChatIDs) > 0 && !p.isRelevantIMDecision(d, searchKeywords, sig) {
				summary := fmt.Sprintf(
					"[Chat][%s] %s: %s",
					d.Status, d.Title, d.Decision)
				summaries = append(summaries, summary)
			}
		}
	}

	// 3. 第三优先级：任意其他决策（补充）
	if len(summaries) < 5 {
		for _, d := range allDecisions {
			if len(summaries) >= 5 {
				break
			}
			if len(d.FeishuLinks.RelatedChatIDs) == 0 &&
				len(d.FeishuLinks.RelatedDocTokens) == 0 {
				summary := fmt.Sprintf(
					"[%s] %s: %s",
					d.Status, d.Title, d.Decision)
				summaries = append(summaries, summary)
			}
		}
	}

	return summaries, nil
}

func (p *IMContextProvider) extractKeywords(sig *StateChangeSignal) []string {
	var keywords []string
	// 从 keywords 中提取
	keywords = append(keywords, sig.Context.Keywords...)
	// 从 ContentSnippet 中提取一些关键词
	if sig.Context.ContentSnippet != "" {
		words := strings.Fields(sig.Context.ContentSnippet)
		if len(words) > 10 {
			words = words[:10] // 取前 10 个词
		}
		keywords = append(keywords, words...)
	}
	return keywords
}

func (p *IMContextProvider) isRelevantIMDecision(
	d *decision.DecisionNode, searchKeywords []string, sig *StateChangeSignal,
) bool {
	// 检查是否有相关的 ChatID
	if len(d.FeishuLinks.RelatedChatIDs) > 0 {
		// 如果当前信号也有相关 IDs，检查是否有交集
		for _, chatID := range d.FeishuLinks.RelatedChatIDs {
			for _, id := range sig.RelatedIDs {
				if id == chatID {
					return true
				}
			}
		}
	}

	// 通过关键词匹配内容
	if len(searchKeywords) > 0 {
		for _, kw := range searchKeywords {
			if strings.Contains(strings.ToLower(d.Title), strings.ToLower(kw)) ||
				strings.Contains(strings.ToLower(d.Decision), strings.ToLower(kw)) {
				return true
			}
		}
	}

	// 没有匹配
	return false
}

// ===== Doc 上下文提供者 =====

// DocContextProvider 提供同文档的修改历史
type DocContextProvider struct {
}

func NewDocContextProvider() *DocContextProvider {
	return &DocContextProvider{}
}

func (p *DocContextProvider) Name() string {
	return "doc"
}

func (p *DocContextProvider) GetRelevantContext(
	sig *StateChangeSignal, allDecisions []*decision.DecisionNode,
) ([]string, error) {
	var summaries []string

	// 从信号中提取相关标识
	searchKeywords := p.extractKeywords(sig)
	docIDs := p.extractDocIDs(sig)

	// 1. 第一优先级：同一文档的历史决策
	for _, d := range allDecisions {
		if len(summaries) >= 5 {
			break
		}
		if p.isRelevantDocDecision(d, docIDs, searchKeywords) {
			summary := fmt.Sprintf(
				"[Doc][%s] %s: %s",
				d.Status, d.Title, d.Decision)
			summaries = append(summaries, summary)
		}
	}

	// 2. 第二优先级：其他 Doc 决策
	if len(summaries) < 5 {
		for _, d := range allDecisions {
			if len(summaries) >= 5 {
				break
			}
			if len(d.FeishuLinks.RelatedDocTokens) > 0 && !p.isRelevantDocDecision(d, docIDs, searchKeywords) {
				summary := fmt.Sprintf(
					"[Doc][%s] %s: %s",
					d.Status, d.Title, d.Decision)
				summaries = append(summaries, summary)
			}
		}
	}

	// 3. 第三优先级：任意决策
	if len(summaries) < 5 {
		for _, d := range allDecisions {
			if len(summaries) >= 5 {
				break
			}
			if len(d.FeishuLinks.RelatedDocTokens) == 0 &&
				len(d.FeishuLinks.RelatedChatIDs) == 0 {
				summary := fmt.Sprintf(
					"[%s] %s: %s",
					d.Status, d.Title, d.Decision)
				summaries = append(summaries, summary)
			}
		}
	}

	return summaries, nil
}

func (p *DocContextProvider) extractKeywords(sig *StateChangeSignal) []string {
	var keywords []string
	keywords = append(keywords, sig.Context.Keywords...)
	if sig.Context.ContentSnippet != "" {
		words := strings.Fields(sig.Context.ContentSnippet)
		if len(words) > 10 {
			words = words[:10]
		}
		keywords = append(keywords, words...)
	}
	return keywords
}

func (p *DocContextProvider) extractDocIDs(sig *StateChangeSignal) []string {
	var ids []string
	// 从 PrimaryID
	if sig.PrimaryID != "" {
		ids = append(ids, sig.PrimaryID)
	}
	// 从 RelatedIDs
	ids = append(ids, sig.RelatedIDs...)
	// 从 EmbeddedURLs
	for _, url := range sig.Context.EmbeddedURLs {
		if url.URLType == "doc" {
			ids = append(ids, url.ExtractedToken)
		}
	}
	return ids
}

func (p *DocContextProvider) isRelevantDocDecision(
	d *decision.DecisionNode, docIDs []string, searchKeywords []string,
) bool {
	// 检查 DocToken 匹配
	for _, token := range d.FeishuLinks.RelatedDocTokens {
		for _, id := range docIDs {
			if token == id {
				return true
			}
		}
	}

	// 通过关键词匹配内容
	if len(searchKeywords) > 0 {
		for _, kw := range searchKeywords {
			if strings.Contains(strings.ToLower(d.Title), strings.ToLower(kw)) ||
				strings.Contains(strings.ToLower(d.Decision), strings.ToLower(kw)) {
				return true
			}
		}
	}

	return false
}

// ===== Wiki 上下文提供者 =====

// WikiContextProvider 提供同知识库节点的历史
type WikiContextProvider struct {
}

func NewWikiContextProvider() *WikiContextProvider {
	return &WikiContextProvider{}
}

func (p *WikiContextProvider) Name() string {
	return "wiki"
}

func (p *WikiContextProvider) GetRelevantContext(
	sig *StateChangeSignal, allDecisions []*decision.DecisionNode,
) ([]string, error) {
	var summaries []string

	// 提取相关标识
	searchKeywords := p.extractKeywords(sig)
	wikiIDs := p.extractWikiIDs(sig)

	// 1. 第一优先级：同一 Wiki 的历史决策
	for _, d := range allDecisions {
		if len(summaries) >= 5 {
			break
		}
		if p.isRelevantWikiDecision(d, wikiIDs, searchKeywords) {
			summary := fmt.Sprintf(
				"[Wiki][%s] %s: %s",
				d.Status, d.Title, d.Decision)
			summaries = append(summaries, summary)
		}
	}

	// 2. 第二优先级：其他 Wiki 决策
	if len(summaries) < 5 {
		for _, d := range allDecisions {
			if len(summaries) >= 5 {
				break
			}
			// Wiki 使用 RelatedDocTokens 存储节点 Token
			if len(d.FeishuLinks.RelatedDocTokens) > 0 && !p.isRelevantWikiDecision(d, wikiIDs, searchKeywords) {
				summary := fmt.Sprintf(
					"[Wiki][%s] %s: %s",
					d.Status, d.Title, d.Decision)
				summaries = append(summaries, summary)
			}
		}
	}

	// 3. 第三优先级：任意决策
	if len(summaries) < 5 {
		for _, d := range allDecisions {
			if len(summaries) >= 5 {
				break
			}
			if len(d.FeishuLinks.RelatedDocTokens) == 0 &&
				len(d.FeishuLinks.RelatedChatIDs) == 0 {
				summary := fmt.Sprintf(
					"[%s] %s: %s",
					d.Status, d.Title, d.Decision)
				summaries = append(summaries, summary)
			}
		}
	}

	return summaries, nil
}

func (p *WikiContextProvider) extractKeywords(sig *StateChangeSignal) []string {
	var keywords []string
	keywords = append(keywords, sig.Context.Keywords...)
	if sig.Context.ContentSnippet != "" {
		words := strings.Fields(sig.Context.ContentSnippet)
		if len(words) > 10 {
			words = words[:10]
		}
		keywords = append(keywords, words...)
	}
	return keywords
}

func (p *WikiContextProvider) extractWikiIDs(sig *StateChangeSignal) []string {
	var ids []string
	if sig.PrimaryID != "" {
		ids = append(ids, sig.PrimaryID)
	}
	ids = append(ids, sig.RelatedIDs...)
	// 从 EmbeddedURLs 提取 wiki tokens
	for _, url := range sig.Context.EmbeddedURLs {
		if url.URLType == "wiki" {
			ids = append(ids, url.ExtractedToken)
		}
	}
	return ids
}

func (p *WikiContextProvider) isRelevantWikiDecision(
	d *decision.DecisionNode, wikiIDs []string, searchKeywords []string,
) bool {
	// 检查 Wiki 节点匹配
	for _, token := range d.FeishuLinks.RelatedDocTokens {
		for _, id := range wikiIDs {
			if token == id {
				return true
			}
		}
	}

	// 关键词匹配
	if len(searchKeywords) > 0 {
		for _, kw := range searchKeywords {
			if strings.Contains(strings.ToLower(d.Title), strings.ToLower(kw)) ||
				strings.Contains(strings.ToLower(d.Decision), strings.ToLower(kw)) {
				return true
			}
		}
	}

	return false
}

// ===== 通用上下文提供者 =====

// GenericContextProvider 通用上下文提供者（其他源）
type GenericContextProvider struct {
}

func NewGenericContextProvider() *GenericContextProvider {
	return &GenericContextProvider{}
}

func (p *GenericContextProvider) Name() string {
	return "generic"
}

func (p *GenericContextProvider) GetRelevantContext(
	sig *StateChangeSignal, allDecisions []*decision.DecisionNode,
) ([]string, error) {
	var summaries []string

	// 返回最近的任意 5 个决策
	for _, d := range allDecisions {
		if len(summaries) >= 5 {
			break
		}
		summary := fmt.Sprintf(
			"[%s] %s: %s",
			d.Status, d.Title, d.Decision)
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// ===== ContextProvider 工厂 =====

// ContextProviderFactory 上下文提供者工厂
type ContextProviderFactory struct {
	providers map[AdapterType]ContextProvider
}

func NewContextProviderFactory() *ContextProviderFactory {
	factory := &ContextProviderFactory{
		providers: make(map[AdapterType]ContextProvider),
	}

	// 注册各类型的提供者
	factory.providers[AdapterIM] = NewIMContextProvider()
	factory.providers[AdapterDocs] = NewDocContextProvider()
	factory.providers[AdapterWiki] = NewWikiContextProvider()
	factory.providers[AdapterCalendar] = NewGenericContextProvider()
	factory.providers[AdapterTask] = NewGenericContextProvider()
	factory.providers[AdapterVC] = NewGenericContextProvider()

	return factory
}

func (f *ContextProviderFactory) GetProvider(adapterType AdapterType) ContextProvider {
	if p, ok := f.providers[adapterType]; ok {
		return p
	}
	return NewGenericContextProvider()
}
