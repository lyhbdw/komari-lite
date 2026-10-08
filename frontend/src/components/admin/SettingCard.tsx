import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import React from "react";
import { useTranslation } from "react-i18next";
import { ChevronDownIcon } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";

interface SettingCardProps {
  title?: string | React.ReactNode;
  description?: string | React.ReactNode;
  children?: React.ReactNode;
  className?: string;
  bordless?: boolean;
  direction?: "row" | "column" | "row-reverse" | "column-reverse";
  onHeaderClick?: () => void;
}

function SettingCard({
  title = "",
  description = "",
  children,
  className = "",
  direction = "column",
  bordless = false,
  onHeaderClick = () => { },
}: SettingCardProps) {
  const actionChild = React.Children.toArray(children).find(
    (child) => React.isValidElement(child) && child.type === Action
  );

  const otherChildren = React.Children.toArray(children).filter(
    (child) => !(React.isValidElement(child) && child.type === Action)
  );

  const flexDir = direction === "row" ? "flex-row" : direction === "row-reverse" ? "flex-row-reverse" : direction === "column-reverse" ? "flex-col-reverse" : "flex-col";

  return (
    <div
      className={
        bordless
          ? "km-setting-card border-0 bg-transparent p-0 flex flex-wrap justify-between items-center " + flexDir + " " + className
          : "km-setting-card rounded-xl border border-border/60 bg-card p-4 sm:p-5 shadow-2xs hover:border-border/80 transition-all duration-150 flex flex-wrap justify-between items-center " + flexDir + " " + className
      }
    >
      <div
        className="w-full flex flex-row justify-between items-center flex-nowrap"
        onClick={onHeaderClick}
      >
        <div className="flex flex-col gap-1 min-h-10 justify-center pr-4">
          <label className="text-sm font-medium tracking-tight text-foreground">
            {title}
          </label>
          {description && (
            <label className="text-xs text-muted-foreground leading-relaxed mt-0.5">
              {description}
            </label>
          )}
        </div>
        {actionChild}
      </div>
      {otherChildren}
    </div>
  );
}

function Action({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}

SettingCard.Action = Action;

export function SettingCardLongTextInput({
  title = "",
  description = "",
  descriptionPlacement = "header",
  label = "",
  defaultValue = "",
  OnSave = () => { },
  onChange,
  autoDisabled = true,
  isSaving,
  bordless = false,
  showSaveButton = true,
}: {
  title?: string;
  description?: string;
  descriptionPlacement?: "header" | "footer";
  label?: string;
  defaultValue?: string;
  OnSave?: (
    value: string,
    textAreaElement: HTMLTextAreaElement,
    buttonElement: HTMLButtonElement
  ) => void | Promise<unknown>;
  onChange?: (e: React.ChangeEvent<HTMLTextAreaElement>) => void;
  autoDisabled?: boolean;
  isSaving?: boolean;
  bordless?: boolean;
  showSaveButton?: boolean;
}) {
  const { t } = useTranslation();
  const [disabled, setDisabled] = React.useState(false);
  const savingState = Boolean(isSaving) || disabled;
  const [value, setValue] = React.useState(defaultValue);
  const textAreaRef = React.useRef<HTMLTextAreaElement>(null);
  const buttonRef = React.useRef<HTMLButtonElement>(null);
  const resolvedLabel = label || t("common.save");

  React.useEffect(() => {
    setValue(defaultValue);
  }, [defaultValue]);

  const handleSave = () => {
    if (autoDisabled) setDisabled(true);
    const result: any =
      textAreaRef.current && buttonRef.current
        ? OnSave(value, textAreaRef.current, buttonRef.current)
        : undefined;
    if (autoDisabled) {
      const promise: Promise<any> = result;
      if (promise && typeof promise.then === "function") {
        promise.finally(() => setDisabled(false)).catch(() => {});
      } else {
        setDisabled(false);
      }
    }
  };

  const handleTextAreaChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setValue(e.target.value);
    onChange?.(e);
  };

  const showFooterDescription =
    descriptionPlacement === "footer" &&
    description !== undefined &&
    description !== null &&
    description !== "";

  return (
    <SettingCard
      title={title}
      description={descriptionPlacement === "footer" ? undefined : description}
      bordless={bordless}
    >
      <div className="w-full mt-2 flex flex-col gap-2 items-start">
        <Textarea
          className="w-full max-w-2xl font-mono text-xs"
          defaultValue={defaultValue}
          value={value}
          onChange={handleTextAreaChange}
          ref={textAreaRef}
          aria-label={typeof title === "string" ? title : undefined}
        />
        {descriptionPlacement === "footer" ? (
          showFooterDescription || showSaveButton ? (
            <div
              className={`w-full flex flex-row items-center gap-3 ${showFooterDescription ? "justify-between" : "justify-end"}`}
            >
              {showFooterDescription ? (
                <label className="min-w-0 flex-1 text-xs text-muted-foreground">
                  {description}
                </label>
              ) : null}
              {showSaveButton ? (
                <Button
                  ref={buttonRef}
                  onClick={handleSave}
                  disabled={savingState}
                  className="cursor-pointer"
                >
                  {resolvedLabel}
                </Button>
              ) : null}
            </div>
          ) : null
        ) : showSaveButton ? (
          <Button
            ref={buttonRef}
            onClick={handleSave}
            disabled={savingState}
            className="cursor-pointer"
          >
            {resolvedLabel}
          </Button>
        ) : null}
      </div>
    </SettingCard>
  );
}

export function SettingCardCollapse({
  title,
  description,
  defaultOpen = false,
  children,
  bordless = false,
}: {
  title?: string;
  description?: string;
  children?: React.ReactNode;
  defaultOpen?: boolean;
  bordless?: boolean;
}) {
  const [open, setOpen] = React.useState(defaultOpen);

  return (
    <SettingCard
      title={title}
      description={description}
      onHeaderClick={() => setOpen(!open)}
      bordless={bordless}
    >
      <SettingCard.Action>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setOpen(!open)}
          aria-expanded={open}
          aria-controls="collapsible-content"
          className="size-8 text-muted-foreground hover:text-foreground cursor-pointer"
        >
          <motion.div
            initial={{ rotate: 0, scale: 1 }}
            animate={{ rotate: open ? 180 : 0, scale: open ? 1.05 : 1 }}
            transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
          >
            <ChevronDownIcon size={16} />
          </motion.div>
        </Button>
      </SettingCard.Action>
      <AnimatePresence>
        {open && (
          <motion.div
            className="w-full pt-1"
            layout
            initial={{ height: 0, opacity: 0, y: -6 }}
            animate={{ height: "auto", opacity: 1, y: 0 }}
            exit={{ height: 0, opacity: 0, y: -6 }}
            transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
            style={{ overflow: "hidden" }}
            id="collapsible-content"
          >
            <div className="border-t border-border/50 my-3" />
            {children}
          </motion.div>
        )}
      </AnimatePresence>
    </SettingCard>
  );
}

SettingCardCollapse.Header = function Header({
  children,
}: {
  children: React.ReactNode;
}) {
  return <div>{children}</div>;
};
