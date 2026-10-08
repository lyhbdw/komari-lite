import { useEffect, useState } from "react";
import { useRPC2Call } from "@/contexts/useRPC2";
import { usePublicInfo } from "@/contexts/usePublicInfo";

const Footer = () => {
  // 格式化 build 时间
  const formatBuildTime = (isoString: string) => {
    const date = new Date(isoString);
    return (
      date.toLocaleString("zh-CN", {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        timeZone: "Asia/Shanghai",
      }) + " (GMT+8)"
    );
  };

  const buildTime =
    typeof __BUILD_TIME__ !== "undefined" ? __BUILD_TIME__ : null;
  const [versionInfo, setVersionInfo] = useState<{
    hash: string;
    version: string;
  } | null>(null);
  const { call } = useRPC2Call();
  const { publicInfo } = usePublicInfo();
  const customFooterHtml = publicInfo?.theme_settings?.customFooterHtml || "";

  useEffect(() => {
    const fetchVersionInfo = async () => {
      try {
        const data = await call("common:getVersion");
        setVersionInfo({ hash: data.hash?.slice(0, 7), version: data.version });
      } catch (error) {
        console.error("Failed to fetch version info:", error);
      }
    };
    fetchVersionInfo();
  }, [call]);

  return (
    <footer className="km-footer footer p-3 border-t border-border/60 text-xs text-muted-foreground">
      {customFooterHtml ? (
        <div className="flex flex-col justify-center items-center gap-1">
          <span
            dangerouslySetInnerHTML={{
              __html: customFooterHtml,
            }}
          />
          <p className="text-muted-foreground">
            Powered by Komari Monitor.
          </p>
        </div>
      ) : (
        <div className="max-w-6xl mx-auto flex flex-col md:flex-row justify-between items-center md:items-start gap-3">
          <div className="flex flex-col gap-1 items-center md:items-start">
            <p className="font-medium text-foreground/80">
              Powered by Komari Monitor.
            </p>
            {buildTime && (
              <p className="text-muted-foreground/70">
                Build Time: {formatBuildTime(buildTime)}
              </p>
            )}
            {versionInfo && (
              <p className="text-muted-foreground/70 font-mono text-[11px]">
                {versionInfo.version} ({versionInfo.hash})
              </p>
            )}
          </div>
        </div>
      )}
    </footer>
  );
};

export default Footer;
