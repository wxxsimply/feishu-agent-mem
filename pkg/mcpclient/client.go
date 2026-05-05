package mcpclient

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client 飞书记忆 MCP 客户端
type Client struct {
	sdkClient *mcp.Client
	session   *mcp.ClientSession
}

// NewClient 创建新的 MCP 客户端
func NewClient() *Client {
	return &Client{
		sdkClient: mcp.NewClient(&mcp.Implementation{
			Name:    "feishu-mem-client",
			Version: "1.0.0",
		}, nil),
	}
}

// ConnectWithTransport 使用传输连接
func (c *Client) ConnectWithTransport(ctx context.Context, transport mcp.ClientTransport) error {
	session, err := c.sdkClient.Connect(ctx, transport, nil)
	if err != nil {
		return err
	}
	c.session = session
	return nil
}

// Close 关闭连接
func (c *Client) Close() error {
	if c.session != nil {
		c.session.Close()
	}
	return nil
}

// ListTools 列出所有工具
func (c *Client) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	if c.session == nil {
		return nil, fmt.Errorf("not connected")
	}
	result, err := c.session.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool 调用工具
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if c.session == nil {
		return nil, fmt.Errorf("not connected")
	}
	return c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

// Search 搜索决策
func (c *Client) Search(ctx context.Context, query string, topic string, limit int) (string, error) {
	args := map[string]interface{}{"query": query, "limit": float64(limit)}
	if topic != "" {
		args["topic"] = topic
	}

	result, err := c.CallTool(ctx, "search", args)
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "", fmt.Errorf("no content returned")
}

// GetTopic 查询议题的所有决策
func (c *Client) GetTopic(ctx context.Context, topic string) (string, error) {
	result, err := c.CallTool(ctx, "topic", map[string]interface{}{"topic": topic})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "", fmt.Errorf("no content returned")
}

// GetDecision 获取单个决策的详细信息
func (c *Client) GetDecision(ctx context.Context, sdrID string) (string, error) {
	result, err := c.CallTool(ctx, "decision", map[string]interface{}{"sdr_id": sdrID})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "", fmt.Errorf("no content returned")
}

// CreateDecision 创建新决策
func (c *Client) CreateDecision(ctx context.Context, title, decision, rationale, topic, phase, impactLevel string) (string, error) {
	args := map[string]interface{}{
		"title":    title,
		"decision": decision,
		"topic":    topic,
	}
	if rationale != "" {
		args["rationale"] = rationale
	}
	if phase != "" {
		args["phase"] = phase
	}
	if impactLevel != "" {
		args["impact_level"] = impactLevel
	}

	result, err := c.CallTool(ctx, "create_decision", args)
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "", fmt.Errorf("no content returned")
}

// UpdateDecision 更新决策
func (c *Client) UpdateDecision(ctx context.Context, sdrID, title, decision, rationale, status string) (string, error) {
	args := map[string]interface{}{"sdr_id": sdrID}
	if title != "" {
		args["title"] = title
	}
	if decision != "" {
		args["decision"] = decision
	}
	if rationale != "" {
		args["rationale"] = rationale
	}
	if status != "" {
		args["status"] = status
	}

	result, err := c.CallTool(ctx, "update_decision", args)
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "", fmt.Errorf("no content returned")
}

// GetTimeline 获取决策历史时间线
func (c *Client) GetTimeline(ctx context.Context) (string, error) {
	result, err := c.CallTool(ctx, "timeline", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "", fmt.Errorf("no content returned")
}

// ListResources 列出所有资源
func (c *Client) ListResources(ctx context.Context) ([]*mcp.Resource, error) {
	if c.session == nil {
		return nil, fmt.Errorf("not connected")
	}
	result, err := c.session.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	return result.Resources, nil
}

// ReadResource 读取资源
func (c *Client) ReadResource(ctx context.Context, uri string) (string, error) {
	if c.session == nil {
		return "", fmt.Errorf("not connected")
	}
	result, err := c.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		return "", err
	}

	if len(result.Contents) > 0 {
		return result.Contents[0].Text, nil
	}

	return "", fmt.Errorf("no content returned")
}
