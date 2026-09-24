/**
 * JSON-RPC 2.0 标准类型定义
 * 基于规范：https://www.jsonrpc.org/specification
 */

/**
 * JSON-RPC 2.0 请求对象
 */
export interface JSONRPC2Request<T = any> {
  /** JSON-RPC 版本，必须为 "2.0" */
  jsonrpc: "2.0";
  /** 调用的方法名 */
  method: string;
  /** 调用参数（可选） */
  params?: T;
  /** 请求ID，如果为空则为通知请求 */
  id?: string | number | null;
}

/**
 * JSON-RPC 2.0 响应对象（成功）
 */
interface JSONRPC2SuccessResponse<T = any> {
  /** JSON-RPC 版本，必须为 "2.0" */
  jsonrpc: "2.0";
  /** 调用结果 */
  result: T;
  /** 请求ID */
  id: string | number | null;
}

/**
 * JSON-RPC 2.0 错误对象
 */
interface JSONRPC2Error {
  /** 错误代码 */
  code: number;
  /** 错误消息 */
  message: string;
  /** 错误详细信息（可选） */
  data?: any;
}

/**
 * JSON-RPC 2.0 响应对象（错误）
 */
interface JSONRPC2ErrorResponse {
  /** JSON-RPC 版本，必须为 "2.0" */
  jsonrpc: "2.0";
  /** 错误信息 */
  error: JSONRPC2Error;
  /** 请求ID */
  id: string | number | null;
}

/**
 * JSON-RPC 2.0 响应联合类型
 */
export type JSONRPC2Response<T = any> = JSONRPC2SuccessResponse<T> | JSONRPC2ErrorResponse;

/**
 * JSON-RPC 2.0 批量请求
 */
export type JSONRPC2BatchRequest = JSONRPC2Request[];

/**
 * JSON-RPC 2.0 批量响应
 */
export type JSONRPC2BatchResponse = JSONRPC2Response[];

/**
 * RPC 连接状态
 */
export const RPC2ConnectionState = {
  DISCONNECTED: "disconnected",
  CONNECTING: "connecting", 
  CONNECTED: "connected",
  RECONNECTING: "reconnecting",
  ERROR: "error",
} as const;

export type RPC2ConnectionStateType = typeof RPC2ConnectionState[keyof typeof RPC2ConnectionState];

/**
 * RPC 连接选项
 */
export interface RPC2ConnectionOptions {
  /** 自动建立连接 */
  autoConnect?: boolean;
  /** 自动重连 */
  autoReconnect?: boolean;
  /** 重连间隔（毫秒） */
  reconnectInterval?: number;
  /** 最大重连次数 */
  maxReconnectAttempts?: number;
  /** 请求超时时间（毫秒） */
  requestTimeout?: number;
  /** 启用心跳包 */
  enableHeartbeat?: boolean;
  /** 心跳包间隔（毫秒） */
  heartbeatInterval?: number;
  /** 自定义headers（仅用于POST请求） */
  headers?: Record<string, string>;
}

/**
 * RPC 调用选项
 */
export interface RPC2CallOptions {
  /** 请求超时时间（毫秒） */
  timeout?: number;
  /** 是否为通知请求（不期望响应） */
  notification?: boolean;
}

/**
 * 事件监听器类型
 */
export interface RPC2EventListeners {
  onConnect?: () => void;
  onDisconnect?: () => void;
  onError?: (error: Error) => void;
  onReconnecting?: (attempt: number) => void;
  onMessage?: (data: any) => void;
}