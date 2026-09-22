import React, { useCallback, useEffect, useState } from "react";
import {
  PingTaskContext,
  type PingTask,
} from "./ping-task-context";

type Response = {
  data: PingTask[];
  message: string;
  status: string;
};

export const PingTaskProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [pingTasks, setPingTasks] = useState<PingTask[] | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    setIsLoading(true);
    setError(null);
    fetch("/api/admin/ping")
      .then((response) => {
        if (!response.ok) throw new Error("Failed to fetch ping tasks");
        return response.json();
      })
      .then((resp: Response) => {
        setPingTasks(resp && Array.isArray(resp.data) ? resp.data : []);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        setIsLoading(false);
      });
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <PingTaskContext.Provider value={{ pingTasks, isLoading, error, refresh }}>
      {children}
    </PingTaskContext.Provider>
  );
};
