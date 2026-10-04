import { useTranslation } from "react-i18next";

const PriceTags = ({
  price = 0,
  billing_cycle = 30,
  currency = "￥",
  expired_at = Date.now() + 30 * 24 * 60 * 60 * 1000,
  tags = "",
  showTags = false,
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

  const tagList = tags ? tags.split(";").map((t) => t.trim()).filter(Boolean) : [];

  return (
    <div className="flex flex-col gap-1 w-full">
      {/* Line 1: Price and Expiration in strict tabular grid */}
      <div className="grid grid-cols-[74px_70px] items-center gap-1.5 flex-nowrap">
        {price !== 0 ? (
          <span className="inline-flex items-center justify-center h-6 px-1.5 rounded-md text-[11px] font-mono font-medium bg-muted/60 text-foreground border border-border/70 w-[74px] truncate shadow-2xs" title={`${price == -1 ? t("common.free") : `${currency}${price}`}/${getCycleLabel()}`}>
            {price == -1 ? t("common.free") : `${currency}${price}`}/{getCycleLabel()}
          </span>
        ) : (
          <span className="w-[74px]" />
        )}
        <span
          className={`inline-flex items-center justify-center h-6 px-1.5 rounded-md text-[11px] font-mono font-medium border w-[70px] shrink-0 shadow-2xs ${
            diffDays <= 7
              ? "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20"
              : diffDays <= 15
              ? "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20"
              : "bg-muted/40 text-muted-foreground border-border/50"
          }`}
          title={getExpiryLabel()}
        >
          {getExpiryLabel()}
        </span>
      </div>

      {/* Line 2: Custom tags (rendered only if explicitly enabled) */}
      {showTags && tagList.length > 0 && (
        <div className="flex items-center gap-1 overflow-hidden flex-nowrap h-5">
          {tagList.map((tag, index) => {
            const text = tag.replace(/<\w+>$/, "");
            return (
              <span
                key={index}
                className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-muted/60 text-muted-foreground border border-border/50 shrink-0 truncate max-w-[120px]"
                title={text}
              >
                {text}
              </span>
            );
          })}
        </div>
      )}
    </div>
  );
};

export default PriceTags;
