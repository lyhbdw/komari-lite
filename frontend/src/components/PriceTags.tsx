import { useTranslation } from "react-i18next";

const PriceTags = ({
  price = 0,
  billing_cycle = 30,
  currency = "￥",
  expired_at = Date.now() + 30 * 24 * 60 * 60 * 1000,
  hidden = false,
}: {
  expired_at?: string | number;
  price?: number;
  billing_cycle?: number;
  currency?: string;
  tags?: string;
  showTags?: boolean;
  hidden?: boolean;
  [key: string]: any;
}) => {
  const [t] = useTranslation();

  if (hidden) return null;

  const expiredDate = new Date(expired_at);
  const now = new Date();
  const diffTime = expiredDate.getTime() - now.getTime();
  const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

  const getCycleLabel = () => {
    if (billing_cycle >= 27 && billing_cycle <= 32) return t("common.monthly");
    if (billing_cycle >= 87 && billing_cycle <= 95) return t("common.quarterly");
    if (billing_cycle >= 175 && billing_cycle <= 185) return t("common.semi_annual");
    if (billing_cycle >= 360 && billing_cycle <= 370) return t("common.annual");
    if (billing_cycle >= 720 && billing_cycle <= 750) return t("common.biennial");
    if (billing_cycle >= 1080 && billing_cycle <= 1150) return t("common.triennial");
    if (billing_cycle >= 1800 && billing_cycle <= 1850) return t("common.quinquennial");
    if (billing_cycle == -1) return t("common.once");
    return `${billing_cycle}天`;
  };

  const getExpiryLabel = () => {
    if (diffDays <= 0) return t("common.expired");
    if (diffDays > 36500) return t("common.long_term");
    return t("common.expired_in", { days: diffDays });
  };

  const isExpired = diffDays <= 0;
  const isExpiringUrgent = diffDays > 0 && diffDays <= 7;
  const isExpiringWarning = diffDays > 7 && diffDays <= 30;
  const isExpiring = isExpired || isExpiringUrgent || isExpiringWarning;

  const priceText =
    price === -1
      ? t("common.free")
      : price !== 0
      ? `${currency}${price}/${getCycleLabel()}`
      : "";

  const fullExpiryTooltip = `${expiredDate.toLocaleDateString("zh-CN")} 到期 (${getExpiryLabel()})`;

  return (
    <div className="flex items-center">
      {isExpiring ? (
        <span
          className={`inline-flex items-center justify-center h-6 px-2 rounded-md text-[11px] font-mono font-medium border shadow-2xs whitespace-nowrap ${
            isExpired || isExpiringUrgent
              ? "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/25"
              : "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/25"
          }`}
          title={fullExpiryTooltip}
        >
          {priceText ? `${priceText} · ${getExpiryLabel()}` : getExpiryLabel()}
        </span>
      ) : priceText ? (
        <span
          className="inline-flex items-center justify-center h-6 px-2 rounded-md text-[11px] font-mono font-medium bg-muted/60 text-foreground/90 border border-border/70 shadow-2xs whitespace-nowrap"
          title={fullExpiryTooltip}
        >
          {priceText}
        </span>
      ) : expired_at ? (
        <span
          className="inline-flex items-center justify-center h-6 px-2 rounded-md text-[11px] font-mono font-medium bg-muted/40 text-muted-foreground border border-border/50 shadow-2xs whitespace-nowrap"
          title={fullExpiryTooltip}
        >
          {getExpiryLabel()}
        </span>
      ) : (
        <span className="text-xs text-muted-foreground/40 font-mono">-</span>
      )}
    </div>
  );
};

export default PriceTags;
