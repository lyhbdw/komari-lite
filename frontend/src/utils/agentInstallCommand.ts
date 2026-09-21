export type AgentInstallOptions = {
  endpoint: string;
  token?: string;
  interval?: string;
  includeNics?: string;
  excludeNics?: string;
};

export function buildAgentInstallArgs(options: AgentInstallOptions) {
  const args = ["-e", options.endpoint];
  const token = options.token?.trim();
  if (token) args.push("-t", token);
  const interval = options.interval?.trim();
  if (interval) {
    const value = Number.parseFloat(interval);
    args.push("-i", Number.isFinite(value) && value >= 1 ? String(value) : "1");
  }

  const includeNics = options.includeNics?.trim();
  if (includeNics) args.push("--include-nics", includeNics);

  const excludeNics = options.excludeNics?.trim();
  if (excludeNics) args.push("--exclude-nics", excludeNics);

  return args;
}
