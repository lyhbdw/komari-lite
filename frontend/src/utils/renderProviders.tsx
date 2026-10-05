import React, { useState } from "react";
import { toast } from "sonner";
import { Eye, EyeOff } from "lucide-react";

interface RenderProviderInputsProps {
  currentProvider: string;
  providerDefs: any;
  providerValues: any;
  translationPrefix?: string;
  title?: string;
  description?: string;
  footer?: React.ReactNode | string;
  setProviderValues: (updater: (prev: any) => any) => void;
  handleSave: (values: any) => Promise<void>;
  t: any;
  isSaving?: boolean;
}

export const renderProviderInputs = ({
  currentProvider,
  providerDefs,
  providerValues,
  translationPrefix,
  footer,
  setProviderValues,
  handleSave,
  t,
  isSaving = false,
}: RenderProviderInputsProps) => {
  if (!currentProvider || !providerDefs[currentProvider]) return null;

  const fields = providerDefs[currentProvider];

  // 统一保存所有字段
  const handleSaveAll = async () => {
    try {
      const finalValues = { ...providerValues };

      // 验证必填字段
      const requiredFields = fields.filter((f: any) => f.required);
      const missingFields = requiredFields.filter((f: any) => {
        const value = finalValues[f.name];
        return (
          value === undefined ||
          value === null ||
          (typeof value === "string" && value.trim() === "")
        );
      });

      if (missingFields.length > 0) {
        const fieldNames = missingFields
          .map((f: any) => t(`${translationPrefix}.${f.name}`, f.name))
          .join(", ");
        toast.error(t("settings.missing_required_fields", { fieldNames }));
        return;
      }

      // 转换数字类型字段
      const processedValues = { ...finalValues };
      fields.forEach((f: any) => {
        const value = processedValues[f.name];
        if (value === undefined || value === null) return;

        if (["int", "int64"].includes(f.type)) {
          const numValue = value === "" ? 0 : Number(value);
          if (isNaN(numValue) || !Number.isInteger(numValue)) {
            toast.error(
              t("settings.invalid_integer", {
                field: t(`${translationPrefix}.${f.name}`, f.name),
              })
            );
            throw new Error(`Invalid integer value for ${f.name}`);
          }
          processedValues[f.name] = numValue;
        } else if (["float32", "float64"].includes(f.type)) {
          const numValue = value === "" ? 0 : Number(value);
          if (isNaN(numValue)) {
            toast.error(
              t("settings.invalid_number", {
                field: t(`${translationPrefix}.${f.name}`, f.name),
              })
            );
            throw new Error(`Invalid float value for ${f.name}`);
          }
          processedValues[f.name] = numValue;
        }
      });

      await handleSave(processedValues);
    } catch (error) {
      console.error("Validation error:", error);
      throw error;
    }
  };

  const updateLocalValue = (fieldName: string, value: any) => {
    setProviderValues((v: any) => ({ ...v, [fieldName]: value }));
  };

  const ProviderFieldItem = ({ f }: { f: any }) => {
    const [showSecret, setShowSecret] = useState(false);
    const fieldTitle = String(t(`${translationPrefix}.${f.name}`, f.name));
    const fieldDescription = f.help
      ? String(t(`${translationPrefix}.${f.name}_help`, f.help))
      : undefined;
    const fieldValue =
      providerValues[f.name] !== undefined
        ? providerValues[f.name]
        : f.default || "";
    const isSensitive =
      f.name.includes("token") ||
      f.name.includes("secret") ||
      f.name.includes("password");
    const isNumber = ["int", "int64", "float32", "float64"].includes(f.type);

    return (
      <div className="flex flex-col justify-between p-3.5 rounded-lg border border-border/60 bg-muted/20">
        <div>
          <div className="flex items-center justify-between gap-2">
            <label
              htmlFor={`field-${f.name}`}
              className="text-xs font-medium text-foreground block"
            >
              {fieldTitle}
            </label>
            <div className="flex items-center gap-1.5">
              {f.required && (
                <span className="text-[10px] font-mono font-medium text-amber-600 dark:text-amber-400 bg-amber-500/10 px-1.5 py-0.2 rounded border border-amber-500/20">
                  必填
                </span>
              )}
              {isSensitive && (
                <button
                  type="button"
                  onClick={() => setShowSecret(!showSecret)}
                  className="text-muted-foreground hover:text-foreground p-0.5 rounded transition-colors cursor-pointer"
                  title={showSecret ? "隐藏敏感内容" : "显示明文内容"}
                >
                  {showSecret ? <EyeOff size={13} /> : <Eye size={13} />}
                </button>
              )}
            </div>
          </div>
          {fieldDescription && (
            <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
              {fieldDescription}
            </p>
          )}
        </div>

        {f.type === "bool" ? (
          <div className="mt-2.5 flex items-center justify-between pt-1">
            <span className="text-xs text-muted-foreground">
              {fieldValue ? "已启用" : "已停用"}
            </span>
            <input
              type="checkbox"
              id={`field-${f.name}`}
              checked={!!fieldValue}
              onChange={(e) => updateLocalValue(f.name, e.target.checked)}
              className="size-4 cursor-pointer accent-foreground"
            />
          </div>
        ) : f.type === "option" && f.options ? (
          <select
            id={`field-${f.name}`}
            value={fieldValue}
            onChange={(e) => updateLocalValue(f.name, e.target.value)}
            className="mt-2.5 w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs cursor-pointer font-mono"
          >
            {f.options.split(",").map((opt: string) => (
              <option key={opt} value={opt}>
                {opt}
              </option>
            ))}
          </select>
        ) : f.type === "richtext" || f.type === "text" ? (
          <textarea
            id={`field-${f.name}`}
            value={String(fieldValue)}
            onChange={(e) => updateLocalValue(f.name, e.target.value)}
            rows={3}
            placeholder={f.default || ""}
            className="mt-2.5 w-full p-2 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono resize-none"
          />
        ) : (
          <input
            id={`field-${f.name}`}
            type={isSensitive && !showSecret ? "password" : isNumber ? "number" : "text"}
            value={String(fieldValue)}
            onChange={(e) => {
              const val = isNumber
                ? e.target.value === ""
                  ? 0
                  : Number(e.target.value)
                : e.target.value;
              updateLocalValue(f.name, val);
            }}
            placeholder={f.default || ""}
            className="mt-2.5 w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
          />
        )}
      </div>
    );
  };

  return (
    <div key={currentProvider} className="space-y-4 pt-1">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {fields.map((f: any) => (
          <ProviderFieldItem key={f.name} f={f} />
        ))}
      </div>

      {footer && (
        <p className="text-xs text-muted-foreground mt-2 block">
          {footer}
        </p>
      )}

      <div className="pt-2 flex justify-end">
        <button
          type="button"
          onClick={handleSaveAll}
          disabled={isSaving}
          className="h-8 px-4 rounded-lg bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs disabled:opacity-50"
        >
          {isSaving ? t("common.saving", "保存中...") : t("common.save", "保存配置")}
        </button>
      </div>
    </div>
  );
};
