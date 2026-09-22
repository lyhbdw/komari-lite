import React from "react";
import { LiveDataContext } from "./live-data-context";

export const useLiveData = () => React.useContext(LiveDataContext);
