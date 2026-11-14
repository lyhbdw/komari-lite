import React from "react";

export type RegionProbe = {
  country?: string;
  status: "ok" | "unknown";
  reason?: string;
  checked_at?: string;
  detected_at?: string;
};

export type RegionFamilyChecks = {
  google_accounts: RegionProbe;
  youtube: RegionProbe;
  youtube_premium: RegionProbe;
};

export type GoogleRegionChecks = {
  ipv4: RegionFamilyChecks;
  ipv6: RegionFamilyChecks;
  checked_at?: string;
};

export type NodeDetail = {
  uuid: string;
  token: string;
  name: string;
  cpu_name: string;
  virtualization: string;
  arch: string;
  cpu_cores: number;
  os: string;
  gpu_name: string;
  ipv4: string;
  ipv6: string;
  region: string;
  mem_total: number;
  swap_total: number;
  disk_total: number;
  version: string;
  weight: number;
  price: number;
  premium: number;
  premium_currency?: string;
  public_remark: string;
  group: string | undefined;
  billing_cycle: number;
  expired_at: string;
  created_at: string;
  updated_at: string;
  online?: boolean;
  google_regions?: GoogleRegionChecks;
  [key: string]: any;
};

export interface NodeDetailsContextType {
  nodeDetail: NodeDetail[];
  isLoading: boolean;
  error: string | null;
  refresh: () => void;
}

export const NodeDetailsContext = React.createContext<
  NodeDetailsContextType | undefined
>(undefined);
