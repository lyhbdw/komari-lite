import React, { useEffect } from "react";
import { AccountContext, type Account } from "./account-context";
import { onUnauthorized } from "@/lib/auth-events";
import { useFetchedResource } from "./useFetchedResource";

export const AccountProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const { data: account, loading, error, refresh } = useFetchedResource<Account | null>({
    url: "/api/me",
    initialData: null,
    errorMessage: "Failed to fetch account data",
    initialLoading: true,
  });

  // 任意 API 返回 401（会话过期）时，立即刷新账户状态。
  // /api/me 会返回 logged_in: false，AdminPanelBar 随之自动弹出登录框。
  useEffect(() => {
    return onUnauthorized(() => {
      void refresh();
    });
  }, [refresh]);

  return (
    <AccountContext.Provider value={{ account, loading, error, refresh }}>
      {children}
    </AccountContext.Provider>
  );
};
