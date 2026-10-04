import { Link, Outlet, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { MessageSquare, Bell, ZapOff } from "lucide-react";

export default function NotificationLayout() {
  const { t } = useTranslation();
  const location = useLocation();

  const tabs = [
    {
      path: "/admin/notification/channels",
      label: t("notification.channels.title", "通知渠道"),
      icon: MessageSquare,
    },
    {
      path: "/admin/notification/general",
      label: t("notification.general.title", "通用事件"),
      icon: Bell,
    },
    {
      path: "/admin/notification/offline",
      label: t("notification.offline.title", "离线报警"),
      icon: ZapOff,
    },
  ];

  return (
    <div className="space-y-4 max-w-4xl km-admin-notification-layout">
      {/* 顶部横向分栏 Tab */}
      <div className="flex items-center gap-1.5 p-1 bg-muted/50 rounded-lg border border-border/60 w-fit">
        {tabs.map((tab) => {
          const isActive = location.pathname.startsWith(tab.path);
          const IconComponent = tab.icon;
          return (
            <Link
              key={tab.path}
              to={tab.path}
              className={`h-8 px-3.5 rounded-lg flex items-center gap-2 text-xs font-medium transition-all select-none ${
                isActive
                  ? "bg-card text-foreground shadow-2xs font-semibold"
                  : "text-muted-foreground hover:text-foreground hover:bg-card/40"
              }`}
            >
              <IconComponent size={14} className={isActive ? "text-foreground" : "text-muted-foreground"} />
              <span>{tab.label}</span>
            </Link>
          );
        })}
      </div>

      <div className="pt-1">
        <Outlet />
      </div>
    </div>
  );
}
