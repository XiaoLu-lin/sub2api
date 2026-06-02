package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildAccountStatusNotificationContentIncludesChangedAndSummary(t *testing.T) {
	content := BuildAccountStatusNotificationContent(
		Account{ID: 2, Name: "bad", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, ErrorMessage: "<expired>", Schedulable: false},
		[]Account{
			{ID: 1, Name: "ok", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
			{ID: 2, Name: "bad", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, ErrorMessage: "<expired>", Schedulable: false},
		},
		time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
	)

	require.Contains(t, content, "账号状态异常")
	require.Contains(t, content, "bad (#2)")
	require.Contains(t, content, "异常账号")
	require.Contains(t, content, "正常账号")
	require.Contains(t, content, "总数")
	require.Contains(t, content, "&lt;expired&gt;")
	require.NotContains(t, content, "<expired>")
}

func TestBuildAccountStatusNotificationTitleIncludesSummary(t *testing.T) {
	title := BuildAccountStatusNotificationTitle(
		Account{ID: 2, Name: "bad"},
		5,
		3,
		1,
		1,
	)

	require.Equal(t, "账号异常 2/5｜正常3 异常1 停用1｜bad", title)
}

func TestPushPlusAccountStatusNotifierSendsPayload(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer server.Close()

	notifier := NewPushPlusAccountStatusNotifier(config.AccountNotificationConfig{
		Enabled:            true,
		PushPlusToken:      "token-1",
		PushPlusTopic:      "group-1",
		PushPlusChannel:    "wechat",
		PushPlusWebhookURL: server.URL,
	}, server.Client())

	err := notifier.NotifyAccountBecameUnhealthy(context.Background(),
		Account{ID: 9, Name: "openai-a", Status: StatusError},
		[]Account{{ID: 9, Name: "openai-a", Status: StatusError}},
	)

	require.NoError(t, err)
	require.Equal(t, "token-1", got["token"])
	require.Equal(t, "group-1", got["topic"])
	require.Equal(t, "wechat", got["channel"])
	require.Equal(t, "html", got["template"])
	title, ok := got["title"].(string)
	require.True(t, ok)
	require.True(t, strings.Contains(title, "账号异常 1/1"))
	require.True(t, strings.Contains(title, "openai-a"))
}
