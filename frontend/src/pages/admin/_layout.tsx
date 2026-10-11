import { Outlet } from "react-router-dom";

import AdminPanelBar from "../../components/admin/AdminPanelBar";
import { AccountProvider } from "@/contexts/AccountContext";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { useEffect, useState } from "react";
import { getEula } from "@/utils/eula";

import { useTranslation } from "react-i18next";
const AdminLayout = () => {
  const { t, i18n } = useTranslation();
  const { settings, loading, error, setSettings } = useSettings();

  const [open, setOpen] = useState(false);
  useEffect(() => {
    if (loading || error || !settings || settings.eula_accepted !== false) {
      setOpen(false);
      return;
    }
    setOpen(true);
  }, [loading, error, settings]);
  return (
    <>
      <Dialog.Root open={open}>
        <Dialog.Content className="sm:max-w-2xl p-6 gap-5 km-admin-eula-dialog">
          <Dialog.Header className="space-y-1.5 text-left">
            <Dialog.Title className="text-base font-semibold leading-none tracking-tight text-foreground">
              {t("eula.title")}
            </Dialog.Title>
          </Dialog.Header>
          <div className="km-admin-eula-content flex flex-col gap-4">
            <div className="max-h-[65vh] overflow-y-auto rounded-lg border border-border/70 bg-muted/30 p-4 text-xs leading-relaxed">
              <pre className="text-wrap font-sans">{getEula(i18n.language)}</pre>
            </div>
            <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row gap-2 justify-end items-center">
              <Button
                variant="outline"
                onClick={() => window.close()}
              >
                {t("eula.reject")}
              </Button>
              <Button
                onClick={async () => {
                  try {
                    await updateSettingsWithToast(
                      { eula_accepted: true },
                      (key) => key
                    );
                    setSettings((prev) => ({
                      ...prev,
                      eula_accepted: true,
                    }));
                    setOpen(false);
                  } catch {
                    setOpen(true);
                  }
                }}
              >
                {t("eula.accept")}
              </Button>
            </Dialog.Footer>
          </div>
        </Dialog.Content>
      </Dialog.Root>
      <AccountProvider>
        <AdminPanelBar content={<Outlet />} />
      </AccountProvider>
    </>
  );
};

export default AdminLayout;
