import React from "react";
import { PingTaskContext, type PingTask } from "./ping-task-context";
import { useFetchedResource } from "./useFetchedResource";

export const PingTaskProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const {
    data: pingTasks,
    loading: isLoading,
    errorString: error,
    refresh,
  } = useFetchedResource<PingTask[] | null>({
    url: "/api/admin/ping",
    initialData: null,
    errorMessage: "Failed to fetch ping tasks",
    transform: (resp) => (resp && Array.isArray(resp.data) ? resp.data : []),
  });

  return (
    <PingTaskContext.Provider value={{ pingTasks, isLoading, error, refresh }}>
      {children}
    </PingTaskContext.Provider>
  );
};
