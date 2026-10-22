package jsonrpc

import (
	"context"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
)

func init() {
	RegisterWithGroupAndMeta("getEffectiveRetention", rpc.RoleAdmin, adminGetEffectiveRetention, &rpc.MethodMeta{Name: "admin:getEffectiveRetention", Summary: "Inspect configured and effective metric retention (read only)", Returns: "RetentionInfo"})
}
func adminGetEffectiveRetention(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	store := metricstore.GetStore()
	if store == nil {
		return nil, rpc.MakeError(rpc.Unavailable, "metric store is not initialized", nil)
	}
	info, err := store.RetentionInfo(ctx)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to inspect retention: "+err.Error(), nil)
	}
	return info, nil
}
