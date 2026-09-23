import { Box, Flex, Text } from "@radix-ui/themes";
import React from "react";

interface UsageBarProps {
  value: number; // Utilization percentage (0–100)
  label: string; // Label for the bar (e.g., "CPU", "Memory", "Disk")
  compact?: boolean; // Whether to show in compact mode (for tables)
  max?: number; // Maximum value for the bar (e.g., total RAM, total disk space)
}

const UsageBar = React.memo(
  ({ value, label, compact = false, max = 100 }: UsageBarProps) => {
    // Ensure value is between 0 and 100
    const clampedValue = Math.min(Math.max(value, 0), max);

    // Determine color based on thresholds (Monochrome with high-usage red alert)
    const isAlert = clampedValue >= 85;
    const barColor = isAlert ? "var(--destructive, #ef4444)" : "var(--foreground)";

    if (compact) {
      return (
        <Box className="km-usage-bar" style={{ width: "100%" }}>
          <Box
            className="km-usage-bar-track"
            style={{
              width: "100%",
              height: "5px",
              backgroundColor: "var(--muted)",
              border: "1px solid var(--border)",
              borderRadius: "9999px",
              overflow: "hidden",
              marginBottom: "2px",
            }}
          >
            <div
              style={{
                height: "100%",
                backgroundColor: barColor,
                borderRadius: "9999px",
                width: "100%",
                transform: `scaleX(${clampedValue / 100})`,
                transformOrigin: "left center",
                transition: "transform 0.5s ease-out",
              }}
            />
          </Box>
          <label color="gray" className="text-sm">
            {clampedValue.toFixed(1)}%
          </label>
        </Box>
      );
    }

    return (
      <Flex direction="column" gap="1" className="km-usage-bar" style={{ width: "100%" }}>
        <Flex justify="between" align="center">
          <Text size="2" color="gray">
            {label}
          </Text>
          <Text size="2" weight="medium">
            {clampedValue.toFixed(1)}%
          </Text>
        </Flex>
        <Box
          className="km-usage-bar-track"
          style={{
            width: "100%",
            height: "6px",
            backgroundColor: "var(--muted)",
            border: "1px solid var(--border)",
            borderRadius: "9999px",
            overflow: "hidden",
          }}
        >
          <div
            style={{
              height: "100%",
              backgroundColor: barColor,
              borderRadius: "9999px",
              width: "100%",
              transform: `scaleX(${clampedValue / 100})`,
              transformOrigin: "left center",
              transition: "transform 0.5s ease-out",
            }}
          />
        </Box>
      </Flex>
    );
  },
);

export default UsageBar;
