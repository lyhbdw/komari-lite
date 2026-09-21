package jsonrpc

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/komari-monitor/komari/database"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/utils/messageSender"
	msfactory "github.com/komari-monitor/komari/utils/messageSender/factory"
)

func init() {
	reg("getMessageSenderProvider", adminGetMessageSender, "Get message sender provider config or templates")
	reg("setMessageSenderProvider", adminSetMessageSender, "Set message sender provider config")
}

func adminGetMessageSender(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Provider string `json:"provider"`
	}
	req.BindParams(&params)
	if params.Provider != "" {
		cfg, err := database.GetMessageSenderConfigByName(params.Provider)
		if err != nil {
			return nil, rpc.MakeError(rpc.NotFound, "Provider not found: "+err.Error(), nil)
		}
		cfg.Addition = redactProviderAddition(cfg.Addition)
		return cfg, nil
	}
	providers := msfactory.GetSenderConfigs()
	if len(providers) == 0 {
		return nil, rpc.MakeError(rpc.NotFound, "No message sender providers found", nil)
	}
	return providers, nil
}

func redactProviderAddition(raw string) string {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return "{}"
	}
	var walk func(any)
	walk = func(node any) {
		switch current := node.(type) {
		case map[string]any:
			for key, child := range current {
				lower := strings.ToLower(key)
				if strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") {
					current[key] = "[redacted]"
				} else {
					walk(child)
				}
			}
		case []any:
			for _, child := range current {
				walk(child)
			}
		}
	}
	walk(value)
	result, _ := json.Marshal(value)
	return string(result)
}

func adminSetMessageSender(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var senderConfig models.MessageSenderProvider
	if err := req.BindParams(&senderConfig); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid configuration: "+err.Error(), nil)
	}
	if senderConfig.Name == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Provider name is required", nil)
	}
	if _, exists := msfactory.GetConstructor(senderConfig.Name); !exists {
		return nil, rpc.MakeError(rpc.NotFound, "Provider not found: "+senderConfig.Name, nil)
	}
	if err := database.SaveMessageSenderConfig(&senderConfig); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to save message sender provider configuration: "+err.Error(), nil)
	}
	method, _ := config.GetAs[string](config.NotificationMethodKey, "none")
	if method == senderConfig.Name {
		if err := messageSender.LoadProvider(senderConfig.Name, senderConfig.Addition); err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Failed to load message sender provider: "+err.Error(), nil)
		}
	}
	return map[string]any{"message": "Message sender provider set successfully"}, nil
}
