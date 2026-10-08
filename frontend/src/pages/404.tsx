import React from "react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";

const NotFound: React.FC = () => {
  const [t] = useTranslation();
  return (
    <div
      className="km-page-404 flex flex-col items-center justify-center min-h-screen p-4 text-center gap-3 bg-background text-foreground"
    >
      <h1 className="text-6xl font-bold tracking-tight">
        404
      </h1>
      <p className="text-base text-muted-foreground">
        {t("page_not_found")}
      </p>
      <Link to="/" className="mt-2">
        <Button variant="secondary">{t("go_to_home")}</Button>
      </Link>
    </div>
  );
};

export default NotFound;