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
        <div className="km-usage-bar w-full">
          <div
            className="km-usage-bar-track w-full h-[5px] bg-muted border border-border rounded-full overflow-hidden mb-0.5"
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
          </div>
          <span className="text-xs text-muted-foreground font-mono tabular-nums">
            {clampedValue.toFixed(1)}%
          </span>
        </div>
      );
    }

    return (
      <div className="km-usage-bar w-full flex flex-col gap-1">
        <div className="flex justify-between items-center text-xs">
          <span className="text-muted-foreground">
            {label}
          </span>
          <span className="font-medium font-mono tabular-nums text-foreground">
            {clampedValue.toFixed(1)}%
          </span>
        </div>
        <div
          className="km-usage-bar-track w-full h-[6px] bg-muted border border-border rounded-full overflow-hidden"
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
        </div>
      </div>
    );
  },
);

export default UsageBar;
