import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useRPC2Call } from "./useRPC2";
import {
  LIVE_DATA_INTERVAL_MS,
  LiveDataContext,
  mergeLiveData,
} from "./live-data-context";

export const LiveDataProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [live_data, setLiveData] = useState<
    import("../types/LiveData").LiveDataResponse | null
  >(null);
  const liveDataRef = useRef<typeof live_data>(null);
  const [showCallout, setShowCallout] = useState(false);
  const refreshCallbacksRef = useRef<
    Set<(data: NonNullable<typeof live_data>) => void>
  >(new Set());
  const { call } = useRPC2Call();

  const onRefresh = useCallback(
    (callback: (data: NonNullable<typeof live_data>) => void) => {
      refreshCallbacksRef.current.add(callback);
      return () => {
        refreshCallbacksRef.current.delete(callback);
      };
    },
    [],
  );

  const notifyRefreshCallbacks = useCallback(
    (data: NonNullable<typeof live_data>) => {
      refreshCallbacksRef.current.forEach((callback) => callback(data));
    },
    [],
  );

  useEffect(() => {
    let timer: number | undefined;
    let stopped = false;
    let running = false;
    const refreshCallbacks = refreshCallbacksRef.current;

    const clearTimer = () => {
      if (timer !== undefined) {
        window.clearTimeout(timer);
        timer = undefined;
      }
    };

    const scheduleNext = () => {
      clearTimer();
      if (!stopped && !document.hidden) {
        timer = window.setTimeout(fetchLatest, LIVE_DATA_INTERVAL_MS);
      }
    };

    const fetchLatest = async () => {
      if (running || stopped || document.hidden) return;
      running = true;
      try {
        const result: Record<string, any> = await call(
          "common:getNodesLatestStatus",
        );
        if (stopped) return;
        const live = mergeLiveData(result, liveDataRef.current);
        if (live !== liveDataRef.current) {
          liveDataRef.current = live;
          setLiveData(live);
          notifyRefreshCallbacks(live);
        }
        setShowCallout(true);
      } catch (e) {
        if (stopped) return;
        console.error("RPC2 获取最新状态失败:", e);
        setShowCallout(false);
      } finally {
        running = false;
        scheduleNext();
      }
    };

    const handleVisibilityChange = () => {
      if (document.hidden) {
        clearTimer();
      } else if (!running) {
        void fetchLatest();
      }
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    if (!document.hidden) void fetchLatest();

    return () => {
      stopped = true;
      refreshCallbacks.clear();
      clearTimer();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [call, notifyRefreshCallbacks]);

  const contextValue = useMemo(
    () => ({ live_data, showCallout, onRefresh }),
    [live_data, showCallout, onRefresh],
  );

  return (
    <LiveDataContext.Provider value={contextValue}>
      {children}
    </LiveDataContext.Provider>
  );
};
