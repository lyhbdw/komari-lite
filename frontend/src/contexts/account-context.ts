import React from "react";

export type Account = {
  logged_in: boolean;
  username: string;
  uuid: string;
  "2fa_enabled": boolean;
};

export interface AccountContextType {
  account: Account | null;
  loading: boolean;
  error: Error | null;
  refresh: () => void;
}

export const AccountContext =
  React.createContext<AccountContextType | undefined>(undefined);
