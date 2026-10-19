import React from "react";
import { PublicInfoContext, type PublicInfo } from "./public-info-context";
import { useFetchedResource } from "./useFetchedResource";

export const PublicInfoProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const {
    data: publicInfo,
    loading: isLoading,
    errorString: error,
    refresh,
  } = useFetchedResource<PublicInfo | null>({
    url: "/api/public",
    initialData: null,
    errorMessage: "Failed to fetch public info",
    transform: (resp) => resp?.data ?? null,
  });

  return (
    <PublicInfoContext.Provider value={{ publicInfo, isLoading, error, refresh }}>
      {children}
    </PublicInfoContext.Provider>
  );
};
