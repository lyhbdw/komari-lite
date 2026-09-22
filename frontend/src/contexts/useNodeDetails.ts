import React from "react";
import { NodeDetailsContext } from "./node-details-context";

export const useNodeDetails = () => {
  const context = React.useContext(NodeDetailsContext);
  if (context === undefined) {
    throw new Error("useNodeDetails must be used within a NodeDetailsProvider");
  }
  return context;
};
