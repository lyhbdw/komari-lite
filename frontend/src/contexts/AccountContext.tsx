import React from "react";
import { AccountContext, type Account } from "./account-context";
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

  return (
    <AccountContext.Provider value={{ account, loading, error, refresh }}>
      {children}
    </AccountContext.Provider>
  );
};
