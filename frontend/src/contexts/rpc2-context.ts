import React from "react";
import { RPC2Client } from "../lib/rpc2";
import type { RPC2ConnectionStateType } from "../types/rpc2";

export interface RPC2ContextType {
  client: RPC2Client;
  connectionState: RPC2ConnectionStateType;
  isConnected: boolean;
  error: string | null;
  connect: () => Promise<void>;
  disconnect: () => void;
}

export const RPC2Context = React.createContext<RPC2ContextType | undefined>(
  undefined,
);

export const rpc2State: {
  singleton: RPC2Client | null;
  refcount: number;
} = { singleton: null, refcount: 0 };
