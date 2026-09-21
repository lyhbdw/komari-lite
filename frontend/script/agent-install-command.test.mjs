import assert from "node:assert/strict";
import { buildAgentInstallArgs } from "../src/utils/agentInstallCommand.ts";

const args = buildAgentInstallArgs({
  endpoint: "https://panel.example",
  token: "generatedAgentToken123",
  interval: "5",
  includeNics: "eth1,pppoe-wan",
  excludeNics: "eth0",
});

assert.deepEqual(args, [
  "-e",
  "https://panel.example",
  "-t",
  "generatedAgentToken123",
  "-i",
  "5",
  "--include-nics",
  "eth1,pppoe-wan",
  "--exclude-nics",
  "eth0",
]);

const defaultArgs = buildAgentInstallArgs({ endpoint: "https://panel.example", token: "generatedAgentToken123" });
assert.deepEqual(defaultArgs, [
  "-e",
  "https://panel.example",
  "-t",
  "generatedAgentToken123",
]);

assert.deepEqual(
  buildAgentInstallArgs({
    endpoint: "https://panel.example",
    token: "generatedAgentToken123",
    interval: "   invalid ",
    includeNics: "  eth 0  ",
    excludeNics: "  ppp0;wan  ",
  }),
  [
    "-e",
    "https://panel.example",
    "-t",
    "generatedAgentToken123",
    "-i",
    "1",
    "--include-nics",
    "eth 0",
    "--exclude-nics",
    "ppp0;wan",
  ],
);

assert.deepEqual(
  buildAgentInstallArgs({
    endpoint: "https://panel.example",
    interval: "   ",
    includeNics: "  ",
    excludeNics: "\t",
  }),
  ["-e", "https://panel.example"],
);

for (const removedFlag of [
  "--disable-auto-update",
  "--ignore-unsafe-cert",
  "--memory-include-cache",
  "--get-ip-addr-from-nic",
  "--gpu",
  "--install-dir",
  "--install-service-name",
  "--include-mountpoint",
  "--month-rotate",
]) {
  assert.equal(defaultArgs.includes(removedFlag), false, removedFlag);
  assert.equal(args.includes(removedFlag), false, removedFlag);
}

console.log("agent install command contract passed");
