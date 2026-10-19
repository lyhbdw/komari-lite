import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  getFlagSrc,
  resolveRegionCode,
} from "../../../theme/src/utils/regionCore";

interface FlagProps {
  flag: string;
  size?: string;
  className?: string;
}

const Flag = React.memo(({ flag, size, className }: FlagProps) => {
  const { t } = useTranslation();
  const resolvedFlagFileName = resolveRegionCode(flag, "UN");
  const imgSrc = getFlagSrc(flag);
  const altText = t("common.region_flag_alt", { name: resolvedFlagFileName });

  return (
    <span
      className={`km-flag self-center ${className !== undefined ? className : "m-2"} ${size ? `w-${size} h-${size}` : "w-6 h-6"}`}
      style={{ display: "inline-flex", alignItems: "center" }}
      aria-label={altText}
    >
      <img
        src={imgSrc}
        alt={altText}
        style={{ width: "100%", height: "100%", objectFit: "contain" }}
      />
    </span>
  );
});

Flag.displayName = "Flag";

export default Flag;
