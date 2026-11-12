package metadata

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateAISettings is shared by test and recognition, without persisting drafts.
func ValidateAISettings(s AISettings) error {
	u, err := url.Parse(strings.TrimSpace(s.BaseURL))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("请填写有效的 HTTP/HTTPS 接口地址（不含凭据、查询参数）")
	}
	if strings.TrimSpace(s.Model) == "" {
		return errors.New("请填写模型名称")
	}
	return nil
}

func (c *AIClient) TestConnection(ctx context.Context) error {
	reply, err := c.Complete(ctx, []ChatMessage{{Role: "user", Content: "Connection test. Reply OK only."}})
	if err != nil {
		return err
	}
	if strings.TrimSpace(reply) == "" {
		return errors.New("模型返回了空内容")
	}
	return nil
}

// AIErrorMessage intentionally excludes request URLs, keys and response bodies.
func AIErrorMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "请求超时，请检查网络或模型响应速度"
	}
	var upstream *UpstreamError
	if errors.As(err, &upstream) {
		switch upstream.Status {
		case 401, 403:
			return "认证失败，请检查 API 密钥和模型权限"
		case 404:
			return "接口或模型不存在，请检查地址和模型名称"
		case 429:
			return "服务限流或额度不足，请检查账户后重试"
		}
		if upstream.Status != 0 {
			return fmt.Sprintf("服务返回 HTTP %d，请稍后重试", upstream.Status)
		}
		if upstream.Network {
			return "网络连接失败，请检查地址、代理或网络后重试"
		}
	}
	return "服务响应无效，未获得可用结果，请检查模型后重试"
}
