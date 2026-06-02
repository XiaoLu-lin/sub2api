package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type RegistrationNotifier interface {
	NotifyUserRegistered(ctx context.Context, user User, totalUsers int, signupSource string) error
}

type noopRegistrationNotifier struct{}

func NewNoopRegistrationNotifier() RegistrationNotifier {
	return noopRegistrationNotifier{}
}

func (noopRegistrationNotifier) NotifyUserRegistered(context.Context, User, int, string) error {
	return nil
}

type PushPlusRegistrationNotifier struct {
	cfg        config.AccountNotificationConfig
	httpClient *http.Client
}

func NewPushPlusRegistrationNotifier(cfg config.AccountNotificationConfig, httpClient *http.Client) RegistrationNotifier {
	if !cfg.Enabled || strings.TrimSpace(cfg.PushPlusToken) == "" {
		return NewNoopRegistrationNotifier()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if strings.TrimSpace(cfg.PushPlusWebhookURL) == "" {
		cfg.PushPlusWebhookURL = "https://www.pushplus.plus/send"
	}
	return &PushPlusRegistrationNotifier{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

func (n *PushPlusRegistrationNotifier) NotifyUserRegistered(ctx context.Context, user User, totalUsers int, signupSource string) error {
	if n == nil {
		return nil
	}
	payload, err := n.buildPayload(user, totalUsers, signupSource)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.PushPlusWebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pushplus returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (n *PushPlusRegistrationNotifier) buildPayload(user User, totalUsers int, signupSource string) (pushPlusPayload, error) {
	token := strings.TrimSpace(n.cfg.PushPlusToken)
	if token == "" {
		return pushPlusPayload{}, fmt.Errorf("pushplus token is required")
	}
	return pushPlusPayload{
		Token:    token,
		Title:    BuildRegistrationNotificationTitle(user, totalUsers, signupSource),
		Content:  BuildRegistrationNotificationContent(user, totalUsers, signupSource, time.Now()),
		Template: "html",
		Topic:    strings.TrimSpace(n.cfg.PushPlusTopic),
		Channel:  strings.TrimSpace(n.cfg.PushPlusChannel),
	}, nil
}

func BuildRegistrationNotificationTitle(user User, totalUsers int, signupSource string) string {
	source := strings.TrimSpace(signupSource)
	if source == "" {
		source = strings.TrimSpace(user.SignupSource)
	}
	if source == "" {
		source = "unknown"
	}
	identity := strings.TrimSpace(user.Email)
	if identity == "" {
		identity = strings.TrimSpace(user.Username)
	}
	if identity == "" && user.ID > 0 {
		identity = fmt.Sprintf("#%d", user.ID)
	}
	if identity == "" {
		identity = "unknown"
	}
	return fmt.Sprintf("新用户注册｜%s｜来源%s｜总数%d", identity, source, totalUsers)
}

func BuildRegistrationNotificationContent(user User, totalUsers int, signupSource string, now time.Time) string {
	source := strings.TrimSpace(signupSource)
	if source == "" {
		source = strings.TrimSpace(user.SignupSource)
	}
	if source == "" {
		source = "unknown"
	}

	var b strings.Builder
	writeBuilderString(&b, "<div style=\"font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#1f2937;line-height:1.55;\">")
	writeBuilderString(&b, "<h2 style=\"margin:0 0 12px;font-size:20px;\">新用户注册</h2>")
	writeBuilderString(&b, "<div style=\"border:1px solid #bfdbfe;background:#eff6ff;border-radius:8px;padding:12px;margin-bottom:12px;\">")
	writeBuilderString(&b, "<div style=\"font-weight:700;margin-bottom:6px;\">注册用户</div>")
	writeBuilderString(&b, "<div>邮箱：")
	writeBuilderString(&b, html.EscapeString(user.Email))
	writeBuilderString(&b, "</div>")
	if strings.TrimSpace(user.Username) != "" {
		writeBuilderString(&b, "<div>用户名：")
		writeBuilderString(&b, html.EscapeString(user.Username))
		writeBuilderString(&b, "</div>")
	}
	writeBuilderString(&b, "<div>来源：")
	writeBuilderString(&b, html.EscapeString(source))
	writeBuilderString(&b, "</div>")
	writeBuilderString(&b, "<div>时间：")
	writeBuilderString(&b, html.EscapeString(now.Format("2006-01-02 15:04:05")))
	writeBuilderString(&b, "</div>")
	writeBuilderString(&b, "</div>")

	writeBuilderString(&b, "<div style=\"background:#dcfce7;border-radius:8px;padding:12px;text-align:center;\">")
	writeBuilderString(&b, "<div style=\"font-size:13px;color:#166534;\">当前注册人数</div>")
	writeBuilderString(&b, "<div style=\"font-size:28px;font-weight:700;color:#14532d;\">")
	writeBuilderString(&b, html.EscapeString(fmt.Sprintf("%d", totalUsers)))
	writeBuilderString(&b, "</div>")
	writeBuilderString(&b, "</div>")
	writeBuilderString(&b, "</div>")
	return b.String()
}
