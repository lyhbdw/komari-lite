import React from "react";
import { NodeDetailsContext, type NodeDetail } from "./node-details-context";
import { useFetchedResource } from "./useFetchedResource";

export const NodeDetailsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const {
    data: nodeDetail,
    loading: isLoading,
    errorString: error,
    refresh,
  } = useFetchedResource<NodeDetail[]>({
    url: "/api/admin/client/list",
    initialData: [],
    errorMessage: "Failed to fetch node details",
    initialLoading: true,
    loadingOnlyFirstTime: true,
  });

  return (
    <NodeDetailsContext.Provider value={{ nodeDetail, isLoading, error, refresh }}>
      {children}
    </NodeDetailsContext.Provider>
  );
};
