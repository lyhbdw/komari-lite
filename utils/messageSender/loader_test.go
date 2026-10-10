package messageSender

import (
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/utils/messageSender/factory"
)

func TestParseTemplateFormatsEventTimeInLocalTimezone(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	eventTime := time.Date(2026, 7, 17, 1, 30, 0, 123456789, time.UTC)
	got := parseTemplate("{{time}}", models.EventMessage{Time: eventTime})
	want := "2026-07-17 09:30:00"
	if got != want {
		t.Fatalf("formatted event time = %q, want %q", got, want)
	}
}

func TestParseTemplateFormatsEventTimeInBeijingTimeWhenLocalIsUTC(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = originalLocal })

	eventTime := time.Date(2026, 9, 27, 5, 35, 27, 0, time.UTC)
	got := parseTemplate("{{time}}", models.EventMessage{Time: eventTime})
	want := "2026-09-27 13:35:27"
	if got != want {
		t.Fatalf("formatted event time = %q, want %q", got, want)
	}
}

func TestParseTemplateFormatsAnyTypedEventField(t *testing.T) {
	for _, tc := range []struct {
		event any
		want  string
	}{
		{event: models.EventMessage{Event: "Offline"}, want: "Offline"},
		{event: models.EventMessage{Event: 42}, want: "42"},
		{event: models.EventMessage{Event: nil}, want: ""},
	} {
		if got := parseTemplate("{{event}}", tc.event); got != tc.want {
			t.Fatalf("parseTemplate with Event=%v = %q, want %q", tc.event, got, tc.want)
		}
	}
}

func TestParseTemplateModernCardFormat(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	tpl := "{{emoji}} <b>Komari 监控告警 · {{event}}</b>\n━━━━━━━━━━━━━━━━━━\n<b>节点名称</b>：<code>{{client}}</code>\n<b>事件详情</b>：{{message}}\n<b>发生时间</b>：<code>{{time}}</code>"
	msg := models.EventMessage{
		Emoji:   "🔴",
		Event:   "Offline",
		Clients: []models.Client{{Name: "HK-BGP-01"}},
		Message: "节点连接已断开",
		Time:    time.Date(2026, 9, 27, 2, 40, 15, 0, time.UTC),
	}
	got := parseTemplate(tpl, msg)
	want := "🔴 <b>Komari 监控告警 · Offline</b>\n━━━━━━━━━━━━━━━━━━\n<b>节点名称</b>：<code>HK-BGP-01</code>\n<b>事件详情</b>：节点连接已断开\n<b>发生时间</b>：<code>2026-09-27 10:40:15</code>"
	if got != want {
		t.Fatalf("parseTemplate result:\n%s\nwant:\n%s", got, want)
	}
}

func Test(t *testing.T) {
	senders := factory.GetAllMessageSenders()
	if len(senders) == 0 {
		t.Error("No message senders found")
		return
	}
	cfg := factory.GetSenderConfigs()
	if len(cfg) == 0 {
		t.Error("No sender configs found")
		return
	}
	LoadProvider("telegram", `{"bot_token":"test-token","chat_id":"test-chat"}`)
	cp := CurrentProvider
	if cp() == nil {
		t.Error("Current provider is nil")
		return
	}
}
