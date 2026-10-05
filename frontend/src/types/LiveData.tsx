export type LiveData = {
    online: string[];
    data: { [key: string]: Record };
};

export type Record = {
  cpu: {
    usage: number;
  };
  ram: {
    used: number;
  };
  swap: {
    used: number;
  };
  load: {
    load1: number;
    load5: number;
    load15: number;
  };
  disk: {
    used: number;
  };
  network: {
    up: number;
    down: number;
    totalUp: number;
    totalDown: number;
  };
  connections: {
    tcp: number;
    udp: number;
  };
  gpu?: {
    count: number;
    average_usage: number;
    detailed_info: {
      name: string;
      memory_total: number;
      memory_used: number;
      utilization: number;
      temperature: number;
    }[];
  };
  uptime: number;
  process: number;
  message: string;
  updated_at: string;
};

export type LiveDataResponse = {
  data: LiveData;
  status: string;
};

export const DEFAULT_LIVE_RECORD: Record = {
  cpu: { usage: 0 },
  ram: { used: 0 },
  swap: { used: 0 },
  load: { load1: 0, load5: 0, load15: 0 },
  disk: { used: 0 },
  network: { up: 0, down: 0, totalUp: 0, totalDown: 0 },
  connections: { tcp: 0, udp: 0 },
  uptime: 0,
  process: 0,
  message: "",
  updated_at: "",
};
