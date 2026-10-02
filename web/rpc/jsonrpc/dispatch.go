package jsonrpc

import (
	"context"

	"github.com/Tumb1er1376/komari-monitor-lite/pkg/rpc"
)

// Dispatch 是所有传输入口的统一分发点：权限校验 → 执行方法。
// ctx 携带可选的取消/超时；meta 为调用者身份元数据（Principal 为权威来源）。
// 始终返回完整的 JsonRpcResponse（包含错误）。
func Dispatch(ctx context.Context, meta *rpc.ContextMeta, req *rpc.JsonRpcRequest) *rpc.JsonRpcResponse {
	if ctx == nil {
		ctx = context.Background()
	}
	if meta == nil {
		meta = &rpc.ContextMeta{Principal: rpc.NewAnonymousPrincipal()}
	}
	// 保证 Principal 与 Permission 字段双向同步(后者用于向后兼容)。
	if meta.Principal == nil {
		if meta.Permission != "" {
			meta.Principal = rpc.PrincipalFromRole(meta.Permission)
		} else {
			meta.Principal = rpc.NewAnonymousPrincipal()
		}
	}
	if meta.Permission == "" {
		meta.Permission = meta.Principal.PrimaryRole()
	}

	// 命名空间权限校验:基于 Principal 的能力集(集合成员语义)。
	if !rpc.CheckPrincipal(meta.Principal, req.Method) {

		return rpc.ErrorResponse(req.ID, rpc.PermissionDenied, "Permission denied", nil)
	}

	return rpc.CallWithContext(rpc.NewContextWithMeta(ctx, meta), req.ID, req.Method, req.Params)
}
