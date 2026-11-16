import React from "react";
import { useState } from "react";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import type { NodeDetail } from "@/contexts/node-details-context";
import { TextField, Button, Dialog, IconButton, Select, Switch } from "@/components/ui/radix-shim";
import { CircleDollarSign } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import Tips from "@/components/ui/tips";
import { SelectOrInput } from "@/components/ui/select-or-input";

const CURRENCY_SELECT_OPTIONS = [
  { label: "¥ 人民币 (CNY / RMB)", value: "¥" },
  { label: "$ 美元 (USD)", value: "$" },
  { label: "€ 欧元 (EUR)", value: "€" },
  { label: "£ 英镑 (GBP)", value: "£" },
  { label: "HK$ 港币 (HKD)", value: "HK$" },
  { label: "JP¥ 日元 (JPY)", value: "JP¥" },
  { label: "₩ 韩元 (KRW)", value: "₩" },
  { label: "S$ 新加坡元 (SGD)", value: "S$" },
  { label: "A$ 澳大利亚元 (AUD)", value: "A$" },
  { label: "C$ 加拿大元 (CAD)", value: "C$" },
  { label: "CHF 瑞士法郎 (CHF)", value: "CHF" },
  { label: "RM 马来西亚林吉特 (MYR)", value: "RM" },
  { label: "฿ 泰铢 (THB)", value: "฿" },
  { label: "₽ 俄罗斯卢布 (RUB)", value: "₽" },
  { label: "NT$ 新台币 (TWD)", value: "NT$" },
];

const getCurrencySelectOptions = (val: string) => {
  if (val && !CURRENCY_SELECT_OPTIONS.some((o) => o.value === val)) {
    return [{ label: `${val} (自定义)`, value: val }, ...CURRENCY_SELECT_OPTIONS];
  }
  return CURRENCY_SELECT_OPTIONS;
};

function BillingButton({
  node,
  trigger,
  open: controlledOpen,
  onOpenChange,
}: {
  node: NodeDetail;
  trigger?: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const { refresh } = useNodeDetails();
  const [internalOpen, setInternalOpen] = useState(false);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : internalOpen;
  const setOpen = (o: boolean) => {
    if (isControlled) {
      onOpenChange?.(o);
    } else {
      setInternalOpen(o);
    }
  };
  const [saving, setSaving] = useState(false);
  const [billingCycle, setBillingCycle] = React.useState<string>(
    node.billing_cycle.toString()
  );
  const [autoRenewal, setAutoRenewal] = React.useState<boolean>(
    node.auto_renewal || false
  );
  const [currency, setCurrency] = React.useState<string>(node.currency || "$");
  const [premiumCurrency, setPremiumCurrency] = React.useState<string>(
    node.premium_currency || "¥"
  );

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setSaving(true);
      const formData = new FormData(e.target as HTMLFormElement);
      const priceValue = (formData.get("price") as string) || "0";
      const premiumValue = (formData.get("premium") as string) || "0";

      const price = parseFloat(priceValue);
      const premium = premiumValue.trim() === "" ? 0 : parseFloat(premiumValue);

      if (isNaN(price) || (price < 0 && price !== -1)) {
        toast.error(t("admin.nodeTable.invalidPrice"));
        return;
      }
      if (isNaN(premium) || premium < 0) {
        toast.error(t("admin.nodeTable.invalidPremium"));
        return;
      }
      const billingCycleValue = parseInt(
        (formData.get("billingCycle") as string) || "30"
      );
      const expiredAtValue = (formData.get("expiredAt") as string) || "";
      const expiredAt = expiredAtValue
        ? new Date(`${expiredAtValue}T00:00:00Z`).toISOString()
        : null;
      const currencyValue = currency || "$";
      const premiumCurrencyValue = premiumCurrency || "¥";

      await fetch(`/api/admin/client/${node.uuid}/edit`, {
        method: "POST",
        body: JSON.stringify({
          price,
          premium,
          premium_currency: premiumCurrencyValue,
          billing_cycle: billingCycleValue,
          expired_at: expiredAt,
          currency: currencyValue,
          auto_renewal: autoRenewal,
        }),
        headers: {
          "Content-Type": "application/json",
        },
      });
      refresh();
      setOpen(false);
    } catch (error) {
      toast.error("Failed to save billing information:" + error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      {trigger !== null && (
        <Dialog.Trigger asChild>
          {trigger !== undefined ? (
            trigger
          ) : (
            <IconButton
              variant="ghost"
              title={t("admin.nodeTable.billing", "账单")}
            >
              <CircleDollarSign size="18" />
            </IconButton>
          )}
        </Dialog.Trigger>
      )}
      <Dialog.Content className="sm:max-w-[460px] p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <CircleDollarSign size={16} className="text-muted-foreground shrink-0" />
            <span>{t("admin.nodeTable.billing", "账单管理")}</span>
            <span className="font-mono text-xs text-muted-foreground font-normal ml-1">
              · {node.name}
            </span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            {t("admin.nodeTable.billingDescription", "配置该节点的采购价格、结算货币、计费周期与续费规则。")}
          </Dialog.Description>
        </Dialog.Header>
        <form onSubmit={handleSave} className="grid gap-3.5 my-0.5">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.price")}
                </label>
                <Tips size="13"><span>{t("admin.nodeTable.priceTips")}</span></Tips>
              </div>
              <TextField.Root name="price" defaultValue={node.price} placeholder="0.00" className="h-9 text-sm" />
            </div>

            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.currency", "价格货币")}
                </label>
                <Tips size="13"><span>{t("admin.nodeTable.currencyTips")}</span></Tips>
              </div>
              <Select.Root value={currency} onValueChange={setCurrency}>
                <Select.Trigger className="w-full h-9 text-sm" />
                <Select.Content>
                  {getCurrencySelectOptions(currency).map((c) => (
                    <Select.Item key={c.value} value={c.value}>
                      {c.label}
                    </Select.Item>
                  ))}
                </Select.Content>
              </Select.Root>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.premium")}
                </label>
                <Tips size="13"><span>{t("admin.nodeTable.premiumTips")}</span></Tips>
              </div>
              <TextField.Root name="premium" defaultValue={node.premium} placeholder="0.00" className="h-9 text-sm" />
            </div>

            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.premiumCurrency", "溢价货币")}
                </label>
                <Tips size="13"><span>{t("admin.nodeTable.premiumCurrencyTips", "溢价独立结算币种，默认为 ¥")}</span></Tips>
              </div>
              <Select.Root value={premiumCurrency} onValueChange={setPremiumCurrency}>
                <Select.Trigger className="w-full h-9 text-sm" />
                <Select.Content>
                  {getCurrencySelectOptions(premiumCurrency).map((c) => (
                    <Select.Item key={c.value} value={c.value}>
                      {c.label}
                    </Select.Item>
                  ))}
                </Select.Content>
              </Select.Root>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.billingCycle")}
                </label>
                <Tips size="13"><span dangerouslySetInnerHTML={{ __html: t("admin.nodeTable.billingCycleTips") }}></span></Tips>
              </div>
              <SelectOrInput
                options={[
                  { label: t("common.monthly"), value: "30" },
                  { label: t("common.quarterly"), value: "92" },
                  { label: t("common.semi_annual"), value: "184" },
                  { label: t("common.annual"), value: "365" },
                  { label: t("common.biennial"), value: "730" },
                  { label: t("common.triennial"), value: "1095" },
                  { label: t("common.quinquennial"), value: "1825" },
                  { label: t("common.once"), value: "-1" },
                ]}
                type="number"
                name="billingCycle"
                value={billingCycle === "0" ? "" : billingCycle}
                onChange={setBillingCycle}
                className="h-9 text-sm"
              />
            </div>

            <div className="grid gap-1.5">
              <div className="flex items-center h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.expiredAt")}
                </label>
              </div>
              <TextField.Root
                name="expiredAt"
                defaultValue={
                  node.expired_at
                    ? new Date(node.expired_at).toISOString().slice(0, 10)
                    : "0001-01-01"
                }
                type="date"
                className="h-9 text-sm"
              >
                <TextField.Slot side="right">
                  <Button
                    type="button"
                    variant="ghost"
                    size="1"
                    onClick={() => {
                      const dateInput = document.querySelector(
                        'input[name="expiredAt"]'
                      ) as HTMLInputElement;
                      if (dateInput) {
                        const futureDate = new Date();
                        futureDate.setFullYear(futureDate.getFullYear() + 200);
                        dateInput.value = futureDate.toISOString().slice(0, 10);
                      }
                    }}
                  >
                    {t("admin.nodeTable.setToLongTerm", "长期")}
                  </Button>
                </TextField.Slot>
              </TextField.Root>
            </div>
          </div>

          <div className="pt-2 border-t border-border/60">
            <div className="flex items-center justify-between py-1">
              <div className="flex flex-col">
                <span className="text-xs font-medium text-foreground">
                  {t("admin.nodeTable.autoRenewal")}
                </span>
                <span className="text-[11px] text-muted-foreground">
                  {t("admin.nodeTable.autoRenewalDescription")}
                </span>
              </div>
              <Switch checked={autoRenewal} onCheckedChange={setAutoRenewal} />
            </div>
          </div>

          <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
            <Dialog.Close asChild>
              <Button variant="outline" type="button">
                {t("common.cancel")}
              </Button>
            </Dialog.Close>
            <Button type="submit" disabled={saving}>
              {t("common.save")}
            </Button>
          </Dialog.Footer>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}


export { BillingButton };
