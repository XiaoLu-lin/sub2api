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

func TestBuildRegistrationNotificationContentIncludesUserAndTotal(t *testing.T) {
	content := BuildRegistrationNotificationContent(
		User{ID: 7, Email: "new@example.com", Username: "<new-user>", SignupSource: "email"},
		12,
		"",
		time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC),
	)

	require.Contains(t, content, "新用户注册")
	require.Contains(t, content, "new@example.com")
	require.Contains(t, content, "&lt;new-user&gt;")
	require.Contains(t, content, "当前注册人数")
	require.Contains(t, content, ">12<")
	require.NotContains(t, content, "<new-user>")
}

func TestBuildRegistrationNotificationTitle(t *testing.T) {
	require.Equal(t,
		"新用户注册｜new@example.com｜来源email｜总数12",
		BuildRegistrationNotificationTitle(User{Email: "new@example.com"}, 12, "email"),
	)
}

func TestPushPlusRegistrationNotifierSendsPayload(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer server.Close()

	notifier := NewPushPlusRegistrationNotifier(config.AccountNotificationConfig{
		Enabled:            true,
		PushPlusToken:      "token-1",
		PushPlusTopic:      "sub2api-alerts",
		PushPlusChannel:    "wechat",
		PushPlusWebhookURL: server.URL,
	}, server.Client())

	err := notifier.NotifyUserRegistered(context.Background(),
		User{ID: 9, Email: "new@example.com", Username: "new-user"},
		12,
		"email",
	)

	require.NoError(t, err)
	require.Equal(t, "token-1", got["token"])
	require.Equal(t, "sub2api-alerts", got["topic"])
	require.Equal(t, "wechat", got["channel"])
	require.Equal(t, "html", got["template"])
	require.Equal(t, "新用户注册｜new@example.com｜来源email｜总数12", got["title"])
	content, ok := got["content"].(string)
	require.True(t, ok)
	require.True(t, strings.Contains(content, "new@example.com"))
}
