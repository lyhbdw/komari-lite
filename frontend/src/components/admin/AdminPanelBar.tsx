import { Cross1Icon, ExitIcon } from "@radix-ui/react-icons";
import {
  Flex,
  IconButton,
} from "@radix-ui/themes";
import { AnimatePresence, motion } from "motion/react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router-dom";
import ColorSwitch from "../ColorSwitch";

import ThemeSwitch from "../ThemeSwitch";
import { useIsMobile } from "@/hooks/use-mobile";
import menuConfig from "../../config/menuConfig.json";
import type { MenuItem } from "../../types/menu";
import { iconMap } from "../../utils/iconHelper";
import { ChevronDownIcon } from "@radix-ui/react-icons";
import { TablerMenu2 } from "../Icones/Tabler";
import LoginDialog from "../Login";
import InlineSvgIcon from "../InlineSvgIcon";
import { useAccount } from "@/contexts/useAccount";
import { usePublicInfo } from "@/contexts/usePublicInfo";
import { useRPC2Call } from "@/contexts/useRPC2";

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
  const [openSubMenus, setOpenSubMenus] = useState<{ [key: string]: boolean }>({});
  const { account } = useAccount();
  const isMobile = useIsMobile();
  const ishttps = window.location.protocol === "https:";
  const [t] = useTranslation();
  const location = useLocation();
  const { publicInfo } = usePublicInfo();

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

  // 根据当前路径展开对应的菜单分组
  useEffect(() => {
    const newState: { [key: string]: boolean } = {};
    mergedBaseMenuItems.forEach((item) => {
      if (item.children) {
        const isMatched = item.children.some((child: MenuItem) => {
          const childPath = child.path.split("?")[0];
          return (
            location.pathname === childPath ||
            (childPath !== "/" && location.pathname.startsWith(childPath + "/"))
          );
        });
        if (isMatched) {
          newState[item.path] = true;
        }
      }
    });
    setOpenSubMenus((prev: Record<string, boolean>) => ({
      ...prev,
      ...newState,
    }));
  }, [location.pathname, mergedBaseMenuItems]);

  // 侧边栏动画变体
  const sidebarVariants = {
    open: {
      width: isMobile ? "100vw" : "240px",
      opacity: 1,
      transition: {
        type: "spring",
        stiffness: 350,
        damping: 32,
      },
    },
    closed: {
      width: 0,
      opacity: isMobile ? 0 : 1,
      transition: {
        type: "spring",
        stiffness: 350,
        damping: 32,
      },
    },
  };

  const contentVariants = {
    open: {
      opacity: isMobile ? 0 : 1,
      x: isMobile ? "100%" : 0,
      transition: { duration: 0.2 },
    },
    closed: {
      opacity: 1,
      x: 0,
      transition: { duration: 0.2 },
    },
  };

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
        <Flex gap="3" align="center">
          <IconButton
            variant="ghost"
            size="2"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            title={t("common.menu_sidebar", "Menu")}
            aria-label={t("common.menu_sidebar", "Menu")}
            className="text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            style={{
              display: isMobile && sidebarOpen ? "none" : "flex",
            }}
          >
            <TablerMenu2 />
          </IconButton>
          <Link to="/admin/servers" className="flex items-center gap-2.5 group">
            <div className="w-6 h-6 rounded-md bg-foreground text-background flex items-center justify-center font-bold text-xs tracking-tighter shadow-2xs group-hover:opacity-90 transition-opacity select-none">
              K
            </div>
            <span className="text-sm font-semibold tracking-tight text-foreground">
              Komari
            </span>
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded-md bg-muted text-muted-foreground border border-border/60 uppercase font-medium">
              Lite
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
        </Flex>

        <Flex gap="2" align="center" className="km-admin-panel-controls">
          {account && !account.logged_in && (
            <LoginDialog
              autoOpen={true}
              showSettings={false}
              onLoginSuccess={() => {
                window.location.reload();
              }}
            />
          )}

          <a
            href="/"
            target="_blank"
            rel="noopener noreferrer"
            className="hidden sm:inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/80 transition-colors"
            title={t("common.home", "View Site")}
          >
            <span>{t("common.home", "监控前端")}</span>
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="11"
              height="11"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="opacity-50"
            >
              <path d="M15 3h6v6" />
              <path d="M10 14L21 3" />
              <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
            </svg>
          </a>

          <div className="h-3.5 w-px bg-border/60 mx-1 hidden sm:block" />

          <ThemeSwitch />
          <ColorSwitch />

          <IconButton
            variant="ghost"
            size="2"
            className="km-admin-panel-account text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors cursor-pointer"
            onClick={logout}
            title={t("common.logout", "Logout")}
            aria-label={t("common.logout", "Logout")}
          >
            <ExitIcon />
          </IconButton>
        </Flex>
      </header>

      {/* Main Workspace (Sidebar + Content) */}
      <div className="flex-1 flex overflow-hidden relative">
        {/* Sidebar */}
        <AnimatePresence>
          <motion.aside
            variants={sidebarVariants}
            initial="closed"
            animate={sidebarOpen ? "open" : "closed"}
            exit="closed"
            className="km-admin-panel-nav border-r border-sidebar-border/70 bg-sidebar/95 backdrop-blur-xs shrink-0"
            style={{
              height: "100%",
              position: isMobile ? "absolute" : "relative",
              zIndex: isMobile ? 20 : 1,
              overflowY: "auto",
              overflowX: "hidden",
            }}
          >
            <div
              className="p-3 flex flex-col justify-between"
              style={{ minHeight: "100%", minWidth: "240px" }}
            >
              {/* Mobile Close Button */}
              {isMobile && (
                <div className="flex justify-end pb-2 mb-2 border-b border-border/50">
                  <IconButton
                    variant="soft"
                    size="1"
                    title={t("common.close_sidebar", "Close menu")}
                    aria-label={t("common.close_sidebar", "Close menu")}
                    onClick={() => setSidebarOpen(false)}
                  >
                    <Cross1Icon />
                  </IconButton>
                </div>
              )}

              {/* Main Navigation Links */}
              <div className="space-y-1">
                {mainItems.map((item: ExtendedMenuItem) => {
                  const isOpen = openSubMenus[item.path];
                  if (item.children && item.children.length) {
                    return (
                      <div key={item.path} className="space-y-0.5">
                        <div
                          className={`group flex items-center justify-between px-3 py-2 rounded-lg text-sm font-medium cursor-pointer transition-all duration-150 ${
                            isOpen
                              ? "text-sidebar-foreground bg-sidebar-accent/50"
                              : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent/40"
                          }`}
                          onClick={() => {
                            setOpenSubMenus((prev) => ({
                              ...prev,
                              [item.path]: !prev[item.path],
                            }));
                          }}
                        >
                          <div className="flex items-center gap-2.5 min-w-0">
                            {renderIcon(
                              item.icon,
                              item.labelKey,
                              "w-4 h-4 shrink-0 flex items-center justify-center text-muted-foreground group-hover:text-sidebar-foreground transition-colors"
                            )}
                            <span className="truncate">
                              {item.rawLabel || t(item.labelKey)}
                            </span>
                          </div>
                          <ChevronDownIcon
                            className="w-4 h-4 opacity-50 transition-transform duration-200"
                            style={{
                              transform: isOpen ? "rotate(180deg)" : "rotate(0deg)",
                            }}
                          />
                        </div>

                        <motion.div
                          initial={{ height: 0, opacity: 0 }}
                          animate={
                            isOpen
                              ? { height: "auto", opacity: 1 }
                              : { height: 0, opacity: 0 }
                          }
                          transition={{ duration: 0.15 }}
                          style={{ overflow: "hidden" }}
                        >
                          <div className="ml-4 pl-3 my-1 border-l border-border/40 flex flex-col gap-0.5">
                            {item.children.map((child: MenuItem) => (
                              <SidebarItem
                                key={child.path}
                                to={child.path}
                                isSubItem={true}
                                icon={renderIcon(
                                  child.icon,
                                  child.labelKey,
                                  "w-3.5 h-3.5 shrink-0 flex items-center justify-center"
                                )}
                                children={
                                  (child as ExtendedMenuItem).rawLabel ||
                                  t(child.labelKey)
                                }
                                onClick={() => isMobile && setSidebarOpen(false)}
                                newTab={child.newTab}
                                reloadDocument={
                                  (child as ExtendedMenuItem).reloadDocument
                                }
                              />
                            ))}
                          </div>
                        </motion.div>
                      </div>
                    );
                  }

                  return (
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
                  );
                })}
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
          </motion.aside>
        </AnimatePresence>

        {/* Main Content Area */}
        <motion.main
          variants={contentVariants}
          animate={sidebarOpen ? "open" : "closed"}
          className="km-admin-panel-content bg-background flex-1 h-full overflow-y-auto"
          style={{
            display: isMobile && sidebarOpen ? "none" : "block",
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
        </motion.main>
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
  isSubItem = false,
}: {
  to: string;
  onClick: () => void;
  icon: ReactNode;
  children: ReactNode;
  newTab?: boolean;
  reloadDocument?: boolean;
  isSubItem?: boolean;
}) => {
  const location = useLocation();
  const isExternalLink = to.startsWith("http://") || to.startsWith("https://");
  const isActive =
    !isExternalLink &&
    to !== "/" &&
    (to.includes("?")
      ? location.pathname + location.search === to
      : location.pathname === to.split("?")[0]);
  const openInNewTab = newTab === true || (isExternalLink && newTab !== false);

  const baseClasses = isSubItem
    ? `group flex items-center gap-2 px-2.5 py-1.5 rounded-md text-[13px] transition-all duration-150 ${
        isActive
          ? "bg-accent/80 text-accent-foreground font-medium"
          : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent/40"
      }`
    : `group flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all duration-150 ${
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
