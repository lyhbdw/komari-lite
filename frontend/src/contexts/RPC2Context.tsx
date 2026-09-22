import React, { useCallback, useEffect, useMemo, useState } from "react";
import { RPC2Client } from "../lib/rpc2";
import i18n from "../i18n/config";
import {
  RPC2Context,
  rpc2State,
} from "./rpc2-context";

export const RPC2Provider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [client] = useState(() => {
    if (!rpc2State.singleton) {
      rpc2State.singleton = new RPC2Client("/api/rpc2", { autoConnect: true });
    }
    return rpc2State.singleton;
  });
  const [connectionState, setConnectionState] = useState(client.state);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    rpc2State.refcount++;
    client.setEventListeners({
      onConnect: () => {
        setConnectionState(client.state);
        setError(null);
      },
      onDisconnect: () => {
        setConnectionState(client.state);
      },
      onError: (err) => {
        setError(err.message);
        setConnectionState(client.state);
      },
      onReconnecting: (attempt) => {
        setConnectionState(client.state);
        console.log(`RPC2 重连尝试 ${attempt}`);
      },
    });

    if (client.state === "disconnected") {
      client.connect().catch((err) => {
        setError(
          err instanceof Error ? err.message : i18n.t("rpc2.connection_failed"),
        );
        setConnectionState(client.state);
      });
    }

    return () => {
      rpc2State.refcount = Math.max(0, rpc2State.refcount - 1);
      if (rpc2State.refcount === 0) {
        client.clearEventListeners();
        client.disconnect();
      }
    };
  }, [client]);

  const connect = useCallback(async () => {
    try {
      setError(null);
      await client.connect();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : i18n.t("rpc2.connection_failed"),
      );
      throw err;
    }
  }, [client]);

  const disconnect = useCallback(() => {
    client.disconnect();
  }, [client]);

  const isConnected = connectionState === "connected";
  const contextValue = useMemo(
    () => ({
      client,
      connectionState,
      isConnected,
      error,
      connect,
      disconnect,
    }),
    [client, connectionState, isConnected, error, connect, disconnect],
  );

  return (
    <RPC2Context.Provider value={contextValue}>
      {children}
    </RPC2Context.Provider>
  );
};
