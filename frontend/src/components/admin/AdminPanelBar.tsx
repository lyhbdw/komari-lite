import { Button } from "@/components/ui/button";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router-dom";

import { useIsMobile } from "@/hooks/use-mobile";
import menuConfig from "../../config/menuConfig.json";
import type { MenuItem } from "../../types/menu";
import { iconMap } from "../../utils/iconHelper";
import LoginDialog from "../Login";
import InlineSvgIcon from "../InlineSvgIcon";
import { useAccount } from "@/contexts/useAccount";
import { usePublicInfo } from "@/contexts/usePublicInfo";
import { useRPC2Call } from "@/contexts/useRPC2";
import { useThemeMode } from "@/hooks/useSystemTheme";
import { Moon, Sun, SunMedium, LogOut, X, Menu } from "lucide-react";

// 将JSON配置转换为类型安全的菜单项数组 (基础静态菜单)
const baseMenuItems = (menuConfig as { menu: MenuItem[] }).menu;

// 扩展的菜单项类型（允许直接提供 rawLabel 而不是多语言 key）
interface ExtendedMenuItem extends MenuItem {
  rawLabel?: string; // 不走 i18n，直接显示
  reloadDocument?: boolean;
}

interface AdminPanelBarProps {
  content: ReactNode;
}

const AdminPanelBar = ({ content }: AdminPanelBarProps) => {
  const { call } = useRPC2Call();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const { account } = useAccount();
  const isMobile = useIsMobile();
  const ishttps = window.location.protocol === "https:";
  const [t] = useTranslation();
  const { publicInfo } = usePublicInfo();
  const { themeMode, toggleThemeMode } = useThemeMode();

  // 获取版本信息
  const [versionInfo, setVersionInfo] = useState<{
    hash: string;
    version: string;
  } | null>(null);

  useEffect(() => {
    const fetchVersionInfo = async () => {
      try {
        const data = await call("common:getVersion");
        setVersionInfo({
          hash: data.hash?.slice(0, 7),
          version: data.version,
        });
      } catch (error) {
        console.error("Failed to fetch version info:", error);
      }
    };

    fetchVersionInfo();
  }, [call]);

  // Handle responsive behavior
  useEffect(() => {
    const handleResize = () => setSidebarOpen(!isMobile);
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [isMobile]);

  const mergedBaseMenuItems: ExtendedMenuItem[] = useMemo(() => {
    return baseMenuItems;
  }, []);

  const bottomItems = useMemo(() => {
    return mergedBaseMenuItems.filter((item) => item.bottom);
  }, [mergedBaseMenuItems]);

  const mainItems = useMemo(() => {
    return mergedBaseMenuItems.filter((item) => !item.bottom);
  }, [mergedBaseMenuItems]);

  // 侧边栏开合：用 CSS transition 实现（替代 motion 弹簧动画）
  const sidebarWidth = sidebarOpen ? (isMobile ? "100vw" : "240px") : 0;
  const sidebarOpacity = sidebarOpen ? 1 : isMobile ? 0 : 1;
  const contentOpacity = sidebarOpen ? (isMobile ? 0 : 1) : 1;

  function logout() {
    // CSRF 防御：登出改为 POST（服务端仅接受 POST）。
    const form = document.createElement("form");
    form.method = "POST";
    form.action = "/api/logout";
    document.body.appendChild(form);
    form.submit();
  }

  const renderIcon = (
    icon: string,
    labelKey: string,
    className?: string,
    active?: boolean
  ) => {
    const link = /^(https?:\/\/|\/|\.\/|\.\.\/)/.test(icon);
    if (link) {
      return (
        <InlineSvgIcon
          src={icon}
          alt={t(labelKey)}
          style={{
            width: 16,
            height: 16,
            objectFit: "contain",
            opacity: active ? 1 : 0.7,
            filter: active ? "none" : "grayscale(20%)",
          }}
          className={className}
          loading="lazy"
        />
      );
    }
    const Cmp = iconMap[icon];
    if (Cmp) {
      return (
        <Cmp
          className={className}
          style={{
            color: active ? "var(--accent-9)" : "currentColor",
          }}
        />
      );
    }
    return (
      <span
        className={className}
        style={{
          width: 8,
          height: 8,
          display: "inline-block",
          borderRadius: 9999,
          background: active ? "var(--accent-9)" : "currentColor",
          opacity: active ? 1 : 0.4,
        }}
      />
    );
  };

  return (
    <div className="km-admin-layout flex flex-col h-screen w-screen overflow-hidden bg-background text-foreground">
      {/* Top Navbar */}
      <header className="km-admin-panel-topbar h-[50px] shrink-0 border-b border-border/70 bg-background/85 backdrop-blur-md px-4 flex items-center justify-between sticky top-0 z-30 transition-colors">
        <div className="flex items-center gap-3">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            title={t("common.menu_sidebar", "Menu")}
            aria-label={t("common.menu_sidebar", "Menu")}
            className="size-8 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            style={{
              display: isMobile && sidebarOpen ? "none" : "flex",
            }}
          >
            <Menu size={16} />
          </Button>
          <Link to="/admin/servers" className="flex items-center gap-2 group">
            <span className="text-sm sm:text-base font-bold tracking-tight text-foreground group-hover:opacity-85 transition-opacity">
              Monitor
            </span>
          </Link>
          {(publicInfo?.version || versionInfo?.version) && (
            <span
              className="text-xs text-muted-foreground/60 font-mono hidden sm:inline-block ml-1 hover:text-muted-foreground transition-colors"
              title={versionInfo?.hash ? `Commit: ${versionInfo.hash}` : undefined}
            >
              v{publicInfo?.version || versionInfo?.version}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2 km-admin-panel-controls">
          {account && !account.logged_in && (
            <LoginDialog
              autoOpen={true}
              showSettings={false}
              trigger={<span className="hidden" />}
              onLoginSuccess={() => {
                window.location.reload();
              }}
            />
          )}

          <Button
            variant="ghost"
            size="icon"
            className="size-8 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            onClick={toggleThemeMode}
            title={
              themeMode === "auto"
                ? t("common.theme_auto", "自动主题")
                : themeMode === "light"
                ? t("common.theme_light", "浅色主题")
                : t("common.theme_dark", "深色主题")
            }
            aria-label="Toggle Theme"
          >
            {themeMode === "auto" ? (
              <SunMedium className="w-4 h-4" />
            ) : themeMode === "light" ? (
              <Sun className="w-4 h-4" />
            ) : (
              <Moon className="w-4 h-4" />
            )}
          </Button>

          <div className="h-3.5 w-px bg-border/60 mx-1 hidden sm:block" />

          <Button
            variant="ghost"
            size="icon"
            className="size-8 km-admin-panel-account text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors cursor-pointer"
            onClick={logout}
            title={t("common.logout", "Logout")}
            aria-label={t("common.logout", "Logout")}
          >
            <LogOut size={16} />
          </Button>
        </div>
      </header>

      {/* Main Workspace (Sidebar + Content) */}
      <div className="flex-1 flex overflow-hidden relative">
        {/* Sidebar */}
          <aside
            className="km-admin-panel-nav border-r border-sidebar-border/70 bg-sidebar/95 backdrop-blur-xs shrink-0 transition-[width,opacity] duration-300 ease-out"
            style={{
              height: "100%",
              position: isMobile ? "absolute" : "relative",
              zIndex: isMobile ? 20 : 1,
              overflowY: "auto",
              overflowX: "hidden",
              width: sidebarWidth,
              opacity: sidebarOpacity,
            }}
          >
            <div
              className="p-3 flex flex-col justify-between"
              style={{ minHeight: "100%", minWidth: "240px" }}
            >
              {/* Mobile Close Button */}
              {isMobile && (
                <div className="flex justify-end pb-2 mb-2 border-b border-border/50">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    title={t("common.close_sidebar", "Close menu")}
                    aria-label={t("common.close_sidebar", "Close menu")}
                    onClick={() => setSidebarOpen(false)}
                  >
                    <X size={14} />
                  </Button>
                </div>
              )}

              {/* Main Navigation Links */}
              <div className="space-y-1">
                {mainItems.map((item: ExtendedMenuItem) => (
                  <SidebarItem
                    key={item.path}
                    to={item.path}
                    icon={renderIcon(
                      item.icon,
                      item.labelKey,
                      "w-4 h-4 shrink-0 flex items-center justify-center"
                    )}
                    children={item.rawLabel || t(item.labelKey)}
                    onClick={() => isMobile && setSidebarOpen(false)}
                    newTab={item.newTab}
                    reloadDocument={item.reloadDocument}
                  />
                ))}
              </div>

              {/* Bottom Docked Items */}
              {bottomItems.length > 0 && (
                <div className="mt-auto pt-3 border-t border-sidebar-border/50 space-y-1">
                  {bottomItems.map((item: ExtendedMenuItem) => (
                    <SidebarItem
                      key={item.path}
                      to={item.path}
                      icon={renderIcon(
                        item.icon,
                        item.labelKey,
                        "w-4 h-4 shrink-0 flex items-center justify-center"
                      )}
                      children={item.rawLabel || t(item.labelKey)}
                      onClick={() => isMobile && setSidebarOpen(false)}
                      newTab={item.newTab}
                      reloadDocument={item.reloadDocument}
                    />
                  ))}
                </div>
              )}
            </div>
          </aside>

        {/* Main Content Area */}
        <main
          className="km-admin-panel-content bg-background flex-1 h-full overflow-y-auto transition-opacity duration-200"
          style={{
            display: isMobile && sidebarOpen ? "none" : "block",
            opacity: contentOpacity,
          }}
        >
          <div className="max-w-7xl mx-auto w-full p-4 sm:p-6 lg:p-8 space-y-6">
            {!ishttps && (
              <div className="rounded-xl border border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-400 p-3.5 flex items-center gap-3 text-sm">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  className="shrink-0 text-amber-600 dark:text-amber-400"
                >
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                  <line x1="12" y1="9" x2="12" y2="13" />
                  <line x1="12" y1="17" x2="12.01" y2="17" />
                </svg>
                <span>{t("warn_https")}</span>
              </div>
            )}
            {content}
          </div>
        </main>
      </div>
    </div>
  );
};

export default AdminPanelBar;

// 侧边栏项目组件
const SidebarItem = ({
  to,
  onClick,
  icon,
  children,
  newTab,
  reloadDocument,
}: {
  to: string;
  onClick: () => void;
  icon: ReactNode;
  children: ReactNode;
  newTab?: boolean;
  reloadDocument?: boolean;
}) => {
  const location = useLocation();
  const isExternalLink = to.startsWith("http://") || to.startsWith("https://");
  const targetPath = to.split("?")[0];
  const isActive =
    !isExternalLink &&
    to !== "/" &&
    (to.includes("?")
      ? location.pathname + location.search === to
      : location.pathname === targetPath ||
        (targetPath !== "/admin" && location.pathname.startsWith(targetPath + "/")));
  const openInNewTab = newTab === true || (isExternalLink && newTab !== false);

  const baseClasses = `group flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all duration-150 ${
    isActive
      ? "bg-accent text-accent-foreground font-medium shadow-2xs"
      : "text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/50"
  }`;

  const contentInner = (
    <>
      <span
        className={`flex items-center justify-center transition-colors ${
          isActive
            ? "text-accent-foreground"
            : "text-muted-foreground group-hover:text-sidebar-foreground"
        }`}
      >
        {icon}
      </span>
      <span className="flex-1 truncate">{children}</span>
      {isExternalLink && (
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="12"
          height="12"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="opacity-40 group-hover:opacity-80 transition-opacity"
        >
          <path d="M15 3h6v6" />
          <path d="M10 14L21 3" />
          <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
        </svg>
      )}
    </>
  );

  if (openInNewTab || reloadDocument) {
    return (
      <a
        href={to}
        onClick={onClick}
        target={openInNewTab ? "_blank" : undefined}
        rel={openInNewTab ? "noopener noreferrer" : undefined}
        className={baseClasses}
      >
        {contentInner}
      </a>
    );
  }

  return (
    <Link to={to} onClick={onClick} className={baseClasses}>
      {contentInner}
    </Link>
  );
};
