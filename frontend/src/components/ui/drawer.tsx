"use client";

import * as React from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";
import { Theme } from "@/components/ui/radix-shim";
import { useSystemTheme } from "@/hooks/useSystemTheme";

// 原生实现的 Drawer（替代 vaul）：底部/侧边滑出面板，支持
// direction、遮罩点击关闭、Escape 关闭，动画用 CSS transition。
// 为兼容原有样式，content 上仍保留 data-vaul-drawer-direction 属性。

type DrawerDirection = "top" | "bottom" | "left" | "right";

interface DrawerContextValue {
  open: boolean;
  setOpen: (open: boolean) => void;
  direction: DrawerDirection;
}

const DrawerContext = React.createContext<DrawerContextValue | null>(null);

function useDrawerContext(): DrawerContextValue {
  const ctx = React.useContext(DrawerContext);
  if (!ctx) throw new Error("Drawer components must be used within <Drawer>");
  return ctx;
}

interface DrawerProps extends React.HTMLAttributes<HTMLDivElement> {
  direction?: DrawerDirection;
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
}

function Drawer({
  direction = "bottom",
  open: controlledOpen,
  defaultOpen = false,
  onOpenChange,
  children,
  ...props
}: DrawerProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = React.useState(defaultOpen);
  const open = controlledOpen ?? uncontrolledOpen;
  const setOpen = React.useCallback(
    (v: boolean) => {
      if (controlledOpen === undefined) setUncontrolledOpen(v);
      onOpenChange?.(v);
    },
    [controlledOpen, onOpenChange]
  );

  const value = React.useMemo(
    () => ({ open, setOpen, direction }),
    [open, setOpen, direction]
  );

  return (
    <DrawerContext.Provider value={value}>
      <div data-slot="drawer" {...props}>
        {children}
      </div>
    </DrawerContext.Provider>
  );
}

interface DrawerTriggerProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean;
}

function DrawerTrigger({ asChild, onClick, children, ...props }: DrawerTriggerProps) {
  const { setOpen } = useDrawerContext();
  const handleClick = (e: React.MouseEvent<HTMLElement>) => {
    (onClick as any)?.(e);
    setOpen(true);
  };
  if (asChild && React.isValidElement(children)) {
    return React.cloneElement(children as React.ReactElement<any>, {
      onClick: handleClick,
    });
  }
  return (
    <button data-slot="drawer-trigger" onClick={handleClick} {...props}>
      {children}
    </button>
  );
}

function DrawerPortal({ children }: { children?: React.ReactNode }) {
  return <>{children}</>;
}

interface DrawerCloseProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean;
}

function DrawerClose({ asChild, onClick, children, ...props }: DrawerCloseProps) {
  const { setOpen } = useDrawerContext();
  const handleClick = (e: React.MouseEvent<HTMLElement>) => {
    (onClick as any)?.(e);
    setOpen(false);
  };
  if (asChild && React.isValidElement(children)) {
    return React.cloneElement(children as React.ReactElement<any>, {
      onClick: handleClick,
    });
  }
  return (
    <button data-slot="drawer-close" onClick={handleClick} {...props}>
      {children}
    </button>
  );
}

function DrawerOverlay({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  const { open, setOpen } = useDrawerContext();
  return (
    <div
      data-slot="drawer-overlay"
      aria-hidden
      onClick={() => setOpen(false)}
      className={cn(
        "km-ui-drawer-overlay fixed inset-0 z-50 bg-black/50 transition-opacity duration-300",
        open ? "opacity-100" : "pointer-events-none opacity-0",
        className
      )}
      {...props}
    />
  );
}

function directionHiddenClass(direction: DrawerDirection): string {
  switch (direction) {
    case "top":
      return "-translate-y-full";
    case "bottom":
      return "translate-y-full";
    case "left":
      return "-translate-x-full";
    case "right":
      return "translate-x-full";
  }
}

function DrawerContent({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  const { open, setOpen, direction } = useDrawerContext();
  const resolvedAppearance = useSystemTheme();

  // 关闭时保持挂载直到滑出动画结束
  const [mounted, setMounted] = React.useState(open);
  React.useEffect(() => {
    if (open) setMounted(true);
  }, [open ]);

  // Escape 关闭 + 打开时锁定 body 滚动
  React.useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [open, setOpen]);

  if (!mounted) return null;

  const content = (
    <Theme appearance={resolvedAppearance} accentColor="gray">
      <DrawerOverlay />
      <div
        role="dialog"
        aria-modal="true"
        data-slot="drawer-content"
        data-vaul-drawer-direction={direction}
        onTransitionEnd={(e) => {
          if (e.target === e.currentTarget && !open) setMounted(false);
        }}
        className={cn(
          "km-ui-drawer-content group/drawer-content bg-background fixed z-50 flex h-auto flex-col transition-transform duration-300 ease-out",
          "data-[vaul-drawer-direction=top]:inset-x-0 data-[vaul-drawer-direction=top]:top-0 data-[vaul-drawer-direction=top]:mb-24 data-[vaul-drawer-direction=top]:max-h-[80vh] data-[vaul-drawer-direction=top]:rounded-b-lg data-[vaul-drawer-direction=top]:border-b",
          "data-[vaul-drawer-direction=bottom]:inset-x-0 data-[vaul-drawer-direction=bottom]:bottom-0 data-[vaul-drawer-direction=bottom]:mt-24 data-[vaul-drawer-direction=bottom]:max-h-[80vh] data-[vaul-drawer-direction=bottom]:rounded-t-lg data-[vaul-drawer-direction=bottom]:border-t",
          "data-[vaul-drawer-direction=right]:inset-y-0 data-[vaul-drawer-direction=right]:right-0 data-[vaul-drawer-direction=right]:w-3/4 data-[vaul-drawer-direction=right]:border-l data-[vaul-drawer-direction=right]:sm:max-w-sm",
          "data-[vaul-drawer-direction=left]:inset-y-0 data-[vaul-drawer-direction=left]:left-0 data-[vaul-drawer-direction=left]:w-3/4 data-[vaul-drawer-direction=left]:border-r data-[vaul-drawer-direction=left]:sm:max-w-sm",
          !open && directionHiddenClass(direction),
          className
        )}
        {...props}
      >
        {direction === "bottom" && (
          <div className="bg-muted mx-auto mt-4 h-2 w-[100px] shrink-0 rounded-full" />
        )}
        {children}
      </div>
    </Theme>
  );

  return <DrawerPortal data-slot="drawer-portal">{createPortal(content, document.body)}</DrawerPortal>;
}

function DrawerHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      data-slot="drawer-header"
      className={cn("km-ui-drawer-header flex flex-col gap-1.5 p-4", className)}
      {...props}
    />
  );
}

function DrawerFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      data-slot="drawer-footer"
      className={cn("mt-auto flex flex-col gap-2 p-4", className)}
      {...props}
    />
  );
}

function DrawerTitle({
  className,
  ...props
}: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h2
      data-slot="drawer-title"
      className={cn("text-foreground font-semibold", className)}
      {...props}
    />
  );
}

function DrawerDescription({
  className,
  ...props
}: React.HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p
      data-slot="drawer-description"
      className={cn("text-muted-foreground text-sm", className)}
      {...props}
    />
  );
}

export {
  Drawer,
  DrawerTrigger,
  DrawerClose,
  DrawerContent,
  DrawerHeader,
  DrawerFooter,
  DrawerTitle,
  DrawerDescription,
};
