import * as React from "react";
import { cn } from "@/lib/utils";
import { Dialog as ShadcnDialog } from "@/components/ui/dialog";
import { Switch as ShadcnSwitch } from "@/components/ui/switch";

export const Theme = ({ children }: any) => <>{children}</>;

export const Box = ({ as: Comp = "div", className, children, ...props }: any) => (
  <Comp className={className} {...props}>
    {children}
  </Comp>
);

export const Flex = ({
  direction = "row",
  justify,
  align,
  gap,
  wrap,
  mt,
  mb,
  my,
  p,
  className,
  children,
  style,
  ...props
}: any) => {
  const dirCls =
    direction === "column"
      ? "flex-col"
      : direction === "column-reverse"
      ? "flex-col-reverse"
      : direction === "row-reverse"
      ? "flex-row-reverse"
      : "flex-row";
  const justifyCls =
    justify === "center"
      ? "justify-center"
      : justify === "end"
      ? "justify-end"
      : justify === "between"
      ? "justify-between"
      : justify === "around"
      ? "justify-around"
      : justify === "start"
      ? "justify-start"
      : "";
  const alignCls =
    align === "center"
      ? "items-center"
      : align === "start"
      ? "items-start"
      : align === "end"
      ? "items-end"
      : align === "baseline"
      ? "items-baseline"
      : "";
  const gapCls =
    gap === "1" ? "gap-1" : gap === "2" ? "gap-2" : gap === "3" ? "gap-3" : gap === "4" ? "gap-4" : gap ? `gap-${gap}` : "";
  const wrapCls = wrap === "wrap" ? "flex-wrap" : "";
  const mtCls = mt === "1" ? "mt-1" : mt === "2" ? "mt-2" : mt === "3" ? "mt-3" : mt === "4" ? "mt-4" : mt ? `mt-${mt}` : "";
  const mbCls = mb === "1" ? "mb-1" : mb === "2" ? "mb-2" : mb === "3" ? "mb-3" : mb === "4" ? "mb-4" : mb ? `mb-${mb}` : "";
  const pCls = p === "1" ? "p-1" : p === "2" ? "p-2" : p === "3" ? "p-3" : p === "4" ? "p-4" : p ? `p-${p}` : "";

  return (
    <div
      className={cn("flex", dirCls, justifyCls, alignCls, gapCls, wrapCls, mtCls, mbCls, pCls, className)}
      style={style}
      {...props}
    >
      {children}
    </div>
  );
};

export const Text = ({
  size,
  weight,
  color,
  className,
  children,
  as: Comp = "span",
  ...props
}: any) => {
  const weightCls =
    weight === "bold"
      ? "font-bold"
      : weight === "medium"
      ? "font-medium"
      : weight === "semibold"
      ? "font-semibold"
      : "";
  const colorCls =
    color === "red"
      ? "text-destructive"
      : color === "gray"
      ? "text-muted-foreground"
      : color === "green"
      ? "text-emerald-600 dark:text-emerald-400"
      : "";
  const sizeCls =
    size === "1"
      ? "text-xs"
      : size === "2"
      ? "text-sm"
      : size === "3"
      ? "text-base"
      : size === "4"
      ? "text-lg"
      : size === "5"
      ? "text-xl"
      : size === "6"
      ? "text-2xl"
      : size === "9"
      ? "text-6xl"
      : "";

  return (
    <Comp className={cn(sizeCls, weightCls, colorCls, className)} {...props}>
      {children}
    </Comp>
  );
};

export const Button = React.forwardRef<HTMLButtonElement, any>(
  ({ variant = "solid", color, size, className, disabled, children, ...props }, ref) => {
    const isDestructive = color === "red" || color === "destructive";
    const variantCls =
      isDestructive
        ? "bg-destructive text-destructive-foreground hover:bg-destructive/90 shadow-2xs"
        : variant === "soft"
        ? "bg-muted text-foreground hover:bg-muted/80 shadow-2xs"
        : variant === "ghost"
        ? "hover:bg-muted text-muted-foreground hover:text-foreground"
        : variant === "outline"
        ? "border border-border bg-background hover:bg-muted hover:text-foreground shadow-2xs"
        : "bg-foreground text-background hover:opacity-90 shadow-2xs";

    const sizeCls =
      size === "1"
        ? "h-7 px-2.5 text-xs rounded-md"
        : size === "3"
        ? "h-11 px-6 text-base rounded-xl"
        : "h-9 px-4 text-sm rounded-lg";

    return (
      <button
        ref={ref}
        disabled={disabled}
        className={cn(
          "inline-flex items-center justify-center font-medium transition-all select-none cursor-pointer disabled:pointer-events-none disabled:opacity-50",
          variantCls,
          sizeCls,
          className
        )}
        {...props}
      >
        {children}
      </button>
    );
  }
);
Button.displayName = "Button";

export const IconButton = React.forwardRef<HTMLButtonElement, any>(
  ({ variant, color, size, className, disabled, children, ...props }, ref) => {
    const isDestructive = color === "red";
    return (
      <button
        ref={ref}
        type="button"
        disabled={disabled}
        className={cn(
          "inline-flex items-center justify-center rounded-md transition-colors cursor-pointer disabled:pointer-events-none disabled:opacity-50",
          isDestructive
            ? "text-destructive hover:bg-destructive/10"
            : variant === "soft"
            ? "bg-muted text-foreground hover:bg-muted/80"
            : "hover:bg-muted text-muted-foreground hover:text-foreground",
          size === "1" ? "size-6 p-1 text-xs" : "size-8 p-1.5",
          className
        )}
        {...props}
      >
        {children}
      </button>
    );
  }
);
IconButton.displayName = "IconButton";

export const Badge = ({ color, variant, size, className, children, ...props }: any) => {
  const colorCls =
    color === "red"
      ? "bg-destructive/10 text-destructive border-destructive/20"
      : color === "green"
      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
      : color === "blue"
      ? "bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/20"
      : color === "amber" || color === "yellow"
      ? "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20"
      : "bg-muted text-muted-foreground border-border";

  return (
    <span
      className={cn("inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-medium border", colorCls, className)}
      {...props}
    >
      {children}
    </span>
  );
};

export const Card = ({ size, className, children, ...props }: any) => (
  <div
    className={cn("rounded-xl border border-border bg-card p-4 shadow-2xs text-card-foreground", className)}
    {...props}
  >
    {children}
  </div>
);

export const Callout = {
  Root: ({ color, className, children, ...props }: any) => (
    <div
      className={cn(
        "flex items-start gap-2.5 p-3 rounded-lg border text-xs",
        color === "red"
          ? "bg-destructive/10 text-destructive border-destructive/20"
          : "bg-muted/50 text-foreground border-border",
        className
      )}
      {...props}
    >
      {children}
    </div>
  ),
  Icon: ({ children }: any) => <div className="shrink-0 mt-0.5">{children}</div>,
  Text: ({ children, className }: any) => <div className={cn("leading-relaxed", className)}>{children}</div>,
};

export const Separator = ({ size, className, ...props }: any) => (
  <div className={cn("shrink-0 bg-border h-px w-full my-2", className)} {...props} />
);

export const Dialog = ShadcnDialog;
export const Switch = ShadcnSwitch;

export const Checkbox = React.forwardRef<HTMLInputElement, any>(
  ({ checked, onCheckedChange, defaultChecked, onChange, className, ...props }, ref) => (
    <input
      ref={ref}
      type="checkbox"
      checked={checked}
      defaultChecked={defaultChecked}
      onChange={(e) => {
        onChange?.(e);
        onCheckedChange?.(e.target.checked);
      }}
      className={cn(
        "size-4 rounded border-border text-foreground focus:ring-1 focus:ring-foreground cursor-pointer accent-foreground",
        className
      )}
      {...props}
    />
  )
);
Checkbox.displayName = "Checkbox";

export const TextArea = React.forwardRef<HTMLTextAreaElement, any>(
  ({ className, ...props }, ref) => (
    <textarea
      ref={ref}
      className={cn(
        "w-full min-h-[80px] rounded-lg border border-input bg-transparent px-3 py-2 text-sm shadow-xs placeholder:text-muted-foreground outline-none transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
        className
      )}
      {...props}
    />
  )
);
TextArea.displayName = "TextArea";

export const TextField = {
  Root: React.forwardRef<HTMLInputElement, any>(
    ({ className, children, placeholder, value, defaultValue, onChange, id, name, type, disabled, autoFocus, ...props }, ref) => {
      if (children) {
        return (
          <div
            className={cn(
              "relative flex items-center w-full h-9 rounded-lg border border-input bg-transparent px-2.5 shadow-xs transition-colors focus-within:border-ring focus-within:ring-1 focus-within:ring-ring",
              disabled && "cursor-not-allowed opacity-50 bg-muted/30",
              className
            )}
          >
            <input
              ref={ref}
              type={type || "text"}
              placeholder={placeholder}
              value={value}
              defaultValue={defaultValue}
              onChange={onChange}
              id={id}
              name={name}
              disabled={disabled}
              autoFocus={autoFocus}
              className="w-full h-full bg-transparent text-sm text-foreground placeholder:text-muted-foreground outline-none border-none p-0 focus:outline-none"
              {...props}
            />
            {children}
          </div>
        );
      }
      return (
        <input
          ref={ref}
          type={type || "text"}
          placeholder={placeholder}
          value={value}
          defaultValue={defaultValue}
          onChange={onChange}
          id={id}
          name={name}
          disabled={disabled}
          autoFocus={autoFocus}
          className={cn(
            "w-full h-9 rounded-lg border border-input bg-transparent px-3 text-sm text-foreground shadow-xs placeholder:text-muted-foreground outline-none transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
            className
          )}
          {...props}
        />
      );
    }
  ),
  Slot: Object.assign(
    ({ className, side, children, ...props }: any) => (
      <div
        className={cn(
          "flex items-center text-muted-foreground shrink-0",
          side === "right" ? "ml-1.5" : "mr-1.5",
          className
        )}
        {...props}
      >
        {children}
      </div>
    ),
    { displayName: "TextFieldSlot" }
  ),
};

const SelectContext = React.createContext<{
  value?: string;
  onValueChange?: (val: string) => void;
  items: Array<{ value: string; label: React.ReactNode }>;
  registerItem: (val: string, label: React.ReactNode) => void;
}>({ items: [], registerItem: () => {} });

export const Select = {
  Root: ({ value: propVal, defaultValue, onValueChange, children }: any) => {
    const [val, setVal] = React.useState(propVal ?? defaultValue ?? "");
    const [items, setItems] = React.useState<Array<{ value: string; label: React.ReactNode }>>([]);
    const actualVal = propVal !== undefined ? propVal : val;
    const registerItem = React.useCallback((value: string, label: React.ReactNode) => {
      setItems((prev) => (prev.some((i) => i.value === value) ? prev : [...prev, { value, label }]));
    }, []);
    const handleChange = (newVal: string) => {
      setVal(newVal);
      onValueChange?.(newVal);
    };
    return (
      <SelectContext.Provider value={{ value: actualVal, onValueChange: handleChange, items, registerItem }}>
        {children}
      </SelectContext.Provider>
    );
  },
  Trigger: ({ id, className }: any) => {
    const { value, onValueChange, items } = React.useContext(SelectContext);
    return (
      <select
        id={id}
        value={value}
        onChange={(e) => onValueChange?.(e.target.value)}
        className={cn(
          "w-full h-8 px-2.5 rounded-lg border border-input bg-transparent text-foreground text-xs shadow-xs outline-none transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring cursor-pointer dark:bg-muted/20",
          className
        )}
      >
        {items.map((item) => (
          <option key={item.value} value={item.value} className="bg-popover text-popover-foreground">
            {typeof item.label === "string" ? item.label : item.value}
          </option>
        ))}
      </select>
    );
  },
  Content: ({ children }: any) => <>{children}</>,
  Item: ({ value, children }: any) => {
    const { registerItem } = React.useContext(SelectContext);
    React.useEffect(() => {
      registerItem(value, children);
    }, [value, children, registerItem]);
    return null;
  },
};

const DropdownContext = React.createContext<{
  open: boolean;
  setOpen: (o: boolean) => void;
}>({ open: false, setOpen: () => {} });

export const DropdownMenu = {
  Root: ({ children }: any) => {
    const [open, setOpen] = React.useState(false);
    const ref = React.useRef<HTMLDivElement>(null);
    React.useEffect(() => {
      const handleClick = (e: MouseEvent) => {
        if (ref.current && !ref.current.contains(e.target as Node)) {
          setOpen(false);
        }
      };
      if (open) document.addEventListener("mousedown", handleClick);
      return () => document.removeEventListener("mousedown", handleClick);
    }, [open]);
    return (
      <DropdownContext.Provider value={{ open, setOpen }}>
        <div ref={ref} className="relative inline-block text-left">
          {children}
        </div>
      </DropdownContext.Provider>
    );
  },
  Trigger: ({ children }: any) => {
    const { open, setOpen } = React.useContext(DropdownContext);
    return (
      <div onClick={() => setOpen(!open)} className="cursor-pointer inline-flex">
        {children}
      </div>
    );
  },
  Content: ({ align = "end", className, children }: any) => {
    const { open } = React.useContext(DropdownContext);
    if (!open) return null;
    return (
      <div
        className={cn(
          "absolute z-50 mt-1.5 min-w-[160px] rounded-xl border border-border/80 bg-popover/95 p-1 text-popover-foreground shadow-xl backdrop-blur-md animate-in fade-in-0 zoom-in-95 duration-150",
          align === "end" ? "right-0" : "left-0",
          className
        )}
      >
        {children}
      </div>
    );
  },
  Item: ({ onClick, className, children }: any) => {
    const { setOpen } = React.useContext(DropdownContext);
    return (
      <div
        onClick={(e) => {
          onClick?.(e);
          setOpen(false);
        }}
        className={cn(
          "relative flex cursor-pointer select-none items-center rounded-md px-2 py-1.5 text-xs outline-none hover:bg-muted hover:text-foreground transition-colors",
          className
        )}
      >
        {children}
      </div>
    );
  },
  Separator: () => <div className="-mx-1 my-1 h-px bg-border" />,
};

const PopoverContext = React.createContext<{
  open: boolean;
  setOpen: (o: boolean) => void;
}>({ open: false, setOpen: () => {} });

export const Popover = {
  Root: ({ open: controlledOpen, onOpenChange, children }: any) => {
    const [uncontrolledOpen, setUncontrolledOpen] = React.useState(false);
    const isOpen = controlledOpen !== undefined ? controlledOpen : uncontrolledOpen;
    const setOpen = (o: boolean) => {
      setUncontrolledOpen(o);
      onOpenChange?.(o);
    };
    const ref = React.useRef<HTMLDivElement>(null);
    React.useEffect(() => {
      const handleClick = (e: MouseEvent) => {
        if (ref.current && !ref.current.contains(e.target as Node)) {
          setOpen(false);
        }
      };
      if (isOpen) document.addEventListener("mousedown", handleClick);
      return () => document.removeEventListener("mousedown", handleClick);
    }, [isOpen]);
    return (
      <PopoverContext.Provider value={{ open: isOpen, setOpen }}>
        <div ref={ref} className="relative inline-block">
          {children}
        </div>
      </PopoverContext.Provider>
    );
  },
  Trigger: ({ children, ...props }: any) => {
    const { open, setOpen } = React.useContext(PopoverContext);
    return (
      <div onClick={() => setOpen(!open)} className="cursor-pointer inline-flex" {...props}>
        {children}
      </div>
    );
  },
  Content: ({ width, className, children, ...props }: any) => {
    const { open } = React.useContext(PopoverContext);
    if (!open) return null;
    return (
      <div
        style={{ width: width || "auto" }}
        className={cn(
          "absolute z-50 mt-1.5 rounded-xl border border-border/80 bg-popover/95 p-3 text-popover-foreground shadow-xl backdrop-blur-md animate-in fade-in-0 zoom-in-95 duration-150",
          className
        )}
        {...props}
      >
        {children}
      </div>
    );
  },
};

const SegmentedContext = React.createContext<{
  value?: string;
  onValueChange?: (val: string) => void;
}>({});

export const SegmentedControl = {
  Root: ({ value, onValueChange, className, children }: any) => (
    <SegmentedContext.Provider value={{ value, onValueChange }}>
      <div
        className={cn(
          "inline-flex items-center p-0.5 rounded-lg bg-muted border border-border/60 text-muted-foreground text-xs",
          className
        )}
      >
        {children}
      </div>
    </SegmentedContext.Provider>
  ),
  Item: ({ value: itemValue, className, children }: any) => {
    const { value, onValueChange } = React.useContext(SegmentedContext);
    const active = value === itemValue;
    return (
      <button
        type="button"
        onClick={() => onValueChange?.(itemValue)}
        className={cn(
          "h-7 px-2.5 rounded-md font-medium transition-all cursor-pointer",
          active ? "bg-card text-foreground shadow-2xs font-semibold" : "hover:text-foreground",
          className
        )}
      >
        {children}
      </button>
    );
  },
};
