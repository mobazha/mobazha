package request

// Context 包含请求相关的上下文信息
type Context struct {
	// 群组上下文（用于群组集市授权）
	GroupPlatform string
	GroupChatID   string

	// 预留未来扩展字段
	// RequestID string   // 请求跟踪 ID
	// UserAgent string   // 用户代理
	// TraceID   string   // 分布式追踪 ID
	// ClientIP  string   // 客户端 IP
}

// NewContext 创建一个新的请求上下文
func NewContext() *Context {
	return &Context{}
}

// WithGroupContext 设置群组上下文
func (c *Context) WithGroupContext(platform, chatID string) *Context {
	c.GroupPlatform = platform
	c.GroupChatID = chatID
	return c
}

// HasGroupContext 检查是否有群组上下文
func (c *Context) HasGroupContext() bool {
	return c != nil && c.GroupPlatform != "" && c.GroupChatID != ""
}
