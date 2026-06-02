package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type AccountStatusNotifier interface {
	NotifyAccountBecameUnhealthy(ctx context.Context, changed Account, accounts []Account) error
}

type noopAccountStatusNotifier struct{}

func NewNoopAccountStatusNotifier() AccountStatusNotifier {
	return noopAccountStatusNotifier{}
}

func (noopAccountStatusNotifier) NotifyAccountBecameUnhealthy(context.Context, Account, []Account) error {
	return nil
}

type PushPlusAccountStatusNotifier struct {
	cfg        config.AccountNotificationConfig
	httpClient *http.Client
}

func NewPushPlusAccountStatusNotifier(cfg config.AccountNotificationConfig, httpClient *http.Client) AccountStatusNotifier {
	if !cfg.Enabled || strings.TrimSpace(cfg.PushPlusToken) == "" {
		return NewNoopAccountStatusNotifier()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if strings.TrimSpace(cfg.PushPlusWebhookURL) == "" {
		cfg.PushPlusWebhookURL = "https://www.pushplus.plus/send"
	}
	return &PushPlusAccountStatusNotifier{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

func ProvideAccountStatusNotifier(cfg *config.Config) AccountStatusNotifier {
	if cfg == nil {
		return NewNoopAccountStatusNotifier()
	}
	return NewPushPlusAccountStatusNotifier(cfg.AccountNotification, nil)
}

func (n *PushPlusAccountStatusNotifier) NotifyAccountBecameUnhealthy(ctx context.Context, changed Account, accounts []Account) error {
	if n == nil {
		return nil
	}
	payload, err := n.buildPayload(changed, accounts)
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

type pushPlusPayload struct {
	Token    string `json:"token"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Template string `json:"template"`
	Topic    string `json:"topic,omitempty"`
	Channel  string `json:"channel,omitempty"`
}

func (n *PushPlusAccountStatusNotifier) buildPayload(changed Account, accounts []Account) (pushPlusPayload, error) {
	token := strings.TrimSpace(n.cfg.PushPlusToken)
	if token == "" {
		return pushPlusPayload{}, fmt.Errorf("pushplus token is required")
	}
	total, active, inactive, errorCount := summarizeAccounts(accounts)
	return pushPlusPayload{
		Token:    token,
		Title:    BuildAccountStatusNotificationTitle(changed, total, active, inactive, errorCount),
		Content:  BuildAccountStatusNotificationContent(changed, accounts, time.Now()),
		Template: "html",
		Topic:    strings.TrimSpace(n.cfg.PushPlusTopic),
		Channel:  strings.TrimSpace(n.cfg.PushPlusChannel),
	}, nil
}

func BuildAccountStatusNotificationTitle(changed Account, total, active, inactive, errorCount int) string {
	return fmt.Sprintf(
		"账号异常 %d/%d｜正常%d 异常%d 停用%d｜%s",
		errorCount+inactive,
		total,
		active,
		errorCount,
		inactive,
		accountShortName(changed),
	)
}

func BuildAccountStatusNotificationContent(changed Account, accounts []Account, now time.Time) string {
	sorted := append([]Account(nil), accounts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Status == sorted[j].Status {
			return sorted[i].ID < sorted[j].ID
		}
		return accountStatusRank(sorted[i].Status) < accountStatusRank(sorted[j].Status)
	})

	var b strings.Builder
	total, active, inactive, errorCount := summarizeAccounts(sorted)
	writeBuilderString(&b, "<div style=\"font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#1f2937;line-height:1.55;\">")
	writeBuilderString(&b, "<h2 style=\"margin:0 0 12px;font-size:20px;\">账号状态异常</h2>")
	writeSummaryCard(&b, changed, total, active, inactive, errorCount, now)
	writeAccountGroup(&b, "异常账号", filterAccountsByStatus(sorted, StatusError))
	writeAccountGroup(&b, "停用账号", filterAccountsByStatus(sorted, StatusDisabled))
	writeAccountGroup(&b, "正常账号", filterAccountsByStatus(sorted, StatusActive))
	writeAccountGroup(&b, "其他状态账号", filterAccountsByOtherStatus(sorted))
	writeBuilderString(&b, "</div>")
	return b.String()
}

func writeSummaryCard(b *strings.Builder, changed Account, total, active, inactive, errorCount int, now time.Time) {
	writeBuilderString(b, "<div style=\"border:1px solid #fecaca;background:#fff7f7;border-radius:8px;padding:12px;margin-bottom:12px;\">")
	writeBuilderString(b, "<div style=\"font-weight:700;margin-bottom:6px;\">触发账号</div>")
	writeBuilderString(b, "<div>")
	writeBuilderString(b, html.EscapeString(accountDisplayName(changed)))
	writeBuilderString(b, "</div><div>状态：<b>")
	writeBuilderString(b, html.EscapeString(changed.Status))
	writeBuilderString(b, "</b></div>")
	if strings.TrimSpace(changed.ErrorMessage) != "" {
		writeBuilderString(b, "<div>原因：")
		writeBuilderString(b, html.EscapeString(changed.ErrorMessage))
		writeBuilderString(b, "</div>")
	}
	writeBuilderString(b, "<div>时间：")
	writeBuilderString(b, html.EscapeString(now.Format("2006-01-02 15:04:05")))
	writeBuilderString(b, "</div>")
	writeBuilderString(b, "</div>")

	writeBuilderString(b, "<div style=\"display:grid;grid-template-columns:repeat(4,1fr);gap:8px;margin-bottom:12px;\">")
	writeMetric(b, "总数", total, "#e5e7eb")
	writeMetric(b, "正常", active, "#bbf7d0")
	writeMetric(b, "异常", errorCount, "#fecaca")
	writeMetric(b, "停用", inactive, "#fde68a")
	writeBuilderString(b, "</div>")
}

func writeMetric(b *strings.Builder, label string, value int, bg string) {
	writeBuilderString(b, "<div style=\"background:")
	writeBuilderString(b, bg)
	writeBuilderString(b, ";border-radius:8px;padding:10px;text-align:center;\">")
	writeBuilderString(b, "<div style=\"font-size:12px;color:#4b5563;\">")
	writeBuilderString(b, html.EscapeString(label))
	writeBuilderString(b, "</div><div style=\"font-size:20px;font-weight:700;\">")
	writeBuilderString(b, html.EscapeString(fmt.Sprintf("%d", value)))
	writeBuilderString(b, "</div></div>")
}

func writeAccountGroup(b *strings.Builder, title string, accounts []Account) {
	if len(accounts) == 0 {
		return
	}
	writeBuilderString(b, "<h3 style=\"margin:16px 0 8px;font-size:16px;\">")
	writeBuilderString(b, html.EscapeString(title))
	writeBuilderString(b, "</h3>")
	for _, acc := range accounts {
		writeAccountCard(b, acc)
	}
}

func writeAccountCard(b *strings.Builder, acc Account) {
	writeBuilderString(b, "<div style=\"border:1px solid #e5e7eb;border-radius:8px;padding:10px;margin-bottom:8px;\">")
	writeBuilderString(b, "<div style=\"font-weight:700;\">")
	writeBuilderString(b, html.EscapeString(accountDisplayName(acc)))
	writeBuilderString(b, "</div>")
	writeBuilderString(b, "<div style=\"color:#4b5563;font-size:13px;\">")
	writeBuilderString(b, html.EscapeString(acc.Platform))
	writeBuilderString(b, " / ")
	writeBuilderString(b, html.EscapeString(acc.Type))
	writeBuilderString(b, " / ")
	writeBuilderString(b, html.EscapeString(acc.Status))
	writeBuilderString(b, " / 可调度：")
	writeBuilderString(b, html.EscapeString(fmt.Sprintf("%t", acc.Schedulable)))
	writeBuilderString(b, "</div>")
	if strings.TrimSpace(acc.ErrorMessage) != "" {
		writeBuilderString(b, "<div style=\"margin-top:6px;color:#991b1b;\">")
		writeBuilderString(b, html.EscapeString(acc.ErrorMessage))
		writeBuilderString(b, "</div>")
	}
	writeBuilderString(b, "</div>")
}

func writeBuilderString(b *strings.Builder, value string) {
	_, _ = b.WriteString(value)
}

func summarizeAccounts(accounts []Account) (total, active, inactive, errorCount int) {
	for _, acc := range accounts {
		total++
		switch acc.Status {
		case StatusActive:
			active++
		case StatusDisabled:
			inactive++
		case StatusError:
			errorCount++
		}
	}
	return total, active, inactive, errorCount
}

func accountStatusRank(status string) int {
	switch status {
	case StatusError:
		return 0
	case StatusDisabled:
		return 1
	case StatusActive:
		return 2
	default:
		return 3
	}
}

func filterAccountsByStatus(accounts []Account, status string) []Account {
	result := make([]Account, 0)
	for _, acc := range accounts {
		if acc.Status == status {
			result = append(result, acc)
		}
	}
	return result
}

func filterAccountsByOtherStatus(accounts []Account) []Account {
	result := make([]Account, 0)
	for _, acc := range accounts {
		if acc.Status != StatusActive && acc.Status != StatusDisabled && acc.Status != StatusError {
			result = append(result, acc)
		}
	}
	return result
}

func accountDisplayName(account Account) string {
	name := strings.TrimSpace(account.Name)
	if name == "" {
		return fmt.Sprintf("#%d", account.ID)
	}
	return fmt.Sprintf("%s (#%d)", name, account.ID)
}

func accountShortName(account Account) string {
	name := strings.TrimSpace(account.Name)
	if name == "" {
		return fmt.Sprintf("#%d", account.ID)
	}
	return name
}
