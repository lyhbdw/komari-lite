import { Flex } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";

const PriceTags = ({
  price = 0,
  billing_cycle = 30,
  currency = "￥",
  expired_at = Date.now() + 30 * 24 * 60 * 60 * 1000,
  tags = "",
  ip4 = "",
  ip6 = "",
  ...props
}: {
  expired_at?: string | number;
  price?: number;
  billing_cycle?: number;
  currency?: string;
  tags?: string;
  ip4?: any;
  ip6?: any;
} & React.ComponentProps<typeof Flex>) => {
  const [t] = useTranslation();

  if (price == 0) {
    return (
      <Flex gap="1.5" {...props} wrap="wrap" className="km-price-tags items-center">
        <CustomTags tags={tags} />
      </Flex>
    );
  }

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
    return `${billing_cycle} ${t("nodeCard.time_day")}`;
  };

  const getExpiryLabel = () => {
    if (diffDays <= 0) return t("common.expired");
    if (diffDays > 36500) return t("common.long_term");
    return t("common.expired_in", { days: diffDays });
  };

  return (
    <Flex gap="1.5" {...props} wrap="wrap" className="km-price-tags items-center">
      {ip4 && (
        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
          <span className="w-1 h-1 rounded-full bg-emerald-500"></span>
          V4
        </span>
      )}

      {ip6 && (
        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
          <span className="w-1 h-1 rounded-full bg-emerald-500"></span>
          V6
        </span>
      )}

      <span className="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-mono font-medium bg-muted text-foreground border border-border/80">
        {price == -1 ? t("common.free") : `${currency}${price}`}/{getCycleLabel()}
      </span>

      <span
        className={`inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-mono font-medium border ${
          diffDays <= 7
            ? "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20"
            : diffDays <= 15
            ? "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20"
            : "bg-muted/60 text-muted-foreground border-border/60"
        }`}
      >
        {getExpiryLabel()}
      </span>

      <CustomTags tags={tags} />
    </Flex>
  );
};

const CustomTags = ({ tags }: { tags?: string }) => {
  if (!tags || tags.trim() === "") {
    return null;
  }
  const tagList = tags.split(";").filter((tag) => tag.trim() !== "");

  return (
    <>
      {tagList.map((tag, index) => {
        const text = tag.replace(/<\w+>$/, "");
        return (
          <span
            key={index}
            className="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-medium bg-muted/60 text-muted-foreground border border-border/50 hover:text-foreground transition-colors"
          >
            {text}
          </span>
        );
      })}
    </>
  );
};

export default PriceTags;
