import { Link, Outlet, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Globe, Shield, Users } from "lucide-react";

export default function SettingLayout() {
  const { t } = useTranslation();
  const location = useLocation();

  const tabs = [
    {
      path: "/admin/settings/site",
      label: t("settings.site.title", "站点设置"),
      icon: Globe,
    },
    {
      path: "/admin/settings/general",
      label: t("settings.general.title", "网络与安全"),
      icon: Shield,
    },
    {
      path: "/admin/settings/sessions",
      label: t("sessions.title", "会话管理"),
      icon: Users,
    },
  ];

  return (
    <div className="space-y-4 max-w-4xl km-admin-settings-layout">
      {/* 顶部横向分栏 Tab */}
      <div className="flex items-center gap-1.5 p-1 bg-muted/50 rounded-xl border border-border/60 w-fit">
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
