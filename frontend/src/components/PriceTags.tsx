import { useTranslation } from "react-i18next";
import {
  EXPIRE_THRESHOLDS,
  getBillingCycleI18nKey,
  getDaysUntilExpired,
  getExpireStatus,
} from "../../../theme/src/utils/billingCore";

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
  hidden?: boolean;
  [key: string]: any;
}) => {
  const [t] = useTranslation();

  if (hidden) return null;

  const expiredDate = new Date(expired_at);
  const diffDays = getDaysUntilExpired(expired_at);
  const status = getExpireStatus(expired_at, EXPIRE_THRESHOLDS.warning);

  const getCycleLabel = () => {
    const i18nKey = getBillingCycleI18nKey(billing_cycle);
    return i18nKey ? t(i18nKey) : `${billing_cycle}天`;
  };

  const getExpiryLabel = () => {
    if (status === "expired") return t("common.expired");
    if (status === "long_term") return t("common.long_term");
    return t("common.expired_in", { days: diffDays });
  };

  const isExpired = status === "expired";
  const isExpiringUrgent = status === "critical";
  const isExpiringWarning = status === "warning";
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
