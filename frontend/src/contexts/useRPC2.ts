import React, { useCallback } from "react";
import i18n from "../i18n/config";
import { RPC2Context } from "./rpc2-context";

export const useRPC2 = () => {
  const context = React.useContext(RPC2Context);
  if (context === undefined) {
    throw new Error(i18n.t("rpc2.provider_required"));
  }
  return context;
};

export const useRPC2Call = () => {
  const { client, isConnected } = useRPC2();

  const call = useCallback(
    <TParams = any, TResult = any>(
      method: string,
      params?: TParams,
      options?: any,
    ): Promise<TResult> => client.call(method, params, options),
    [client],
  );

  const callViaWebSocket = useCallback(
    <TParams = any, TResult = any>(
      method: string,
      params?: TParams,
      options?: any,
    ): Promise<TResult> => client.callViaWebSocket(method, params, options),
    [client],
  );

  const callViaHTTP = useCallback(
    <TParams = any, TResult = any>(
      method: string,
      params?: TParams,
      options?: any,
    ): Promise<TResult> => client.callViaHTTP(method, params, options),
    [client],
  );

  const batchCall = useCallback(
    (requests: Array<{ method: string; params?: any; notification?: boolean }>) =>
      client.batchCall(requests),
    [client],
  );

  return {
    call,
    callViaWebSocket,
    callViaHTTP,
    batchCall,
    isConnected,
  };
};
