/**
 * 全局认证事件总线。
 *
 * 任何 API 层收到 401（会话过期/未登录）时调用 notifyUnauthorized()，
 * AccountProvider 订阅该事件后刷新 /api/me，账户状态翻转为未登录，
 * AdminPanelBar 中的 LoginDialog 便会自动弹出，用户无需手动刷新页面。
 */

const AUTH_UNAUTHORIZED_EVENT = "komari:unauthorized";

/** API 返回 401 时调用，通知全局刷新登录态 */
export function notifyUnauthorized(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(AUTH_UNAUTHORIZED_EVENT));
  }
}

/** 订阅 401 事件，返回取消订阅函数 */
export function onUnauthorized(listener: () => void): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }
  window.addEventListener(AUTH_UNAUTHORIZED_EVENT, listener);
  return () => window.removeEventListener(AUTH_UNAUTHORIZED_EVENT, listener);
}

/** 会话过期时给用户看的提示（替代各页面千篇一律的 "Failed to fetch settings"） */
export const SESSION_EXPIRED_MESSAGE = "登录已过期，请重新登录";
