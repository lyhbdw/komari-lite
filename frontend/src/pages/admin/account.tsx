import React from "react";

import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { useAccount } from "@/contexts/useAccount";
import {
  Button,
  Dialog,
  Flex,
  Skeleton,
  TextField,
} from "@radix-ui/themes";
import Loading from "@/components/loading";

const Account = () => <InnerLayout />;

const InnerLayout = () => {
  const { t } = useTranslation();
  const { account, loading, error } = useAccount();
  const [usernameSaving, setUsernameSaving] = React.useState(false);
  const [passwordSaving, setPasswordSaving] = React.useState(false);
  const [passwordTwoFa, setPasswordTwoFa] = React.useState("");
  if (loading) {
    return <Loading />;
  }
  if (error) {
    return <div>{error.message}</div>;
  }

  function handleSubmitUsernameChange(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setUsernameSaving(true);
    fetch("/api/admin/update/user", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        uuid: account?.uuid,
        username: (event.currentTarget as HTMLFormElement).username.value,
      }),
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error("Failed to update username");
        }
        return response.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setUsernameSaving(false);
      });
  }
  function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget as HTMLFormElement;
    const password = form.password.value;
    const password_repeat = form.password_repeat.value;
    if (!password || !password_repeat) {
      toast.error(t("account.password_empty_error"));
      return;
    }
    if (password !== password_repeat) {
      toast.error(t("account.password_mismatch_error"));
      return;
    }
    if (password.length < 8) {
      toast.error(t("account.password_too_short_error"));
      return;
    }
    if (!/(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/.test(password)) {
      toast.error(t("account.password_strength_error"));
      return;
    }
    if (account?.["2fa_enabled"] && !passwordTwoFa) {
      toast.error(t("account.otp_empty_error"));
      return;
    }
    setPasswordSaving(true);
    fetch("/api/admin/update/user", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        uuid: account?.uuid,
        password: password,
        "2fa_code": passwordTwoFa,
      }),
    })
      .then(async (response) => {
        if (!response.ok) {
          const data = await response.json();
          throw new Error(data.message || "Failed to update password");
        }
        return response.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
        setPasswordTwoFa("");
        setTimeout(() => {
          window.location.href = "/";
        }, 2000);
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setPasswordSaving(false);
      });
  }
  
  return (
    <div className="km-page-admin-account max-w-4xl space-y-6">
      <div className="space-y-1">
        <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">{t("account.title")}</h1>
        <p className="text-sm text-muted-foreground">
          {t("account.greeting", { username: account?.username })}
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Username form */}
        <div className="rounded-lg border border-border bg-card p-5 shadow-2xs space-y-4">
          <div>
            <h2 className="text-sm font-semibold tracking-tight text-foreground">
              {t("account.change_username_title")}
            </h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              修改您在管理面板的登录用户名。
            </p>
          </div>
          <form
            className="km-account-profile-form space-y-3"
            onSubmit={handleSubmitUsernameChange}
          >
            <TextField.Root
              className="w-full"
              id="username"
              name="username"
              defaultValue={account?.username}
            />
            <div className="pt-1">
              <Button disabled={usernameSaving} type="submit" variant="solid" className="cursor-pointer">
                {t("account.change_username_button")}
              </Button>
            </div>
          </form>
        </div>

        {/* 2FA section */}
        <div className="rounded-lg border border-border bg-card p-5 shadow-2xs space-y-4">
          <div>
            <h2 className="text-sm font-semibold tracking-tight text-foreground">2FA 两步验证</h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              提高账户安全性，登录时要求验证码。
            </p>
          </div>
          <div className="km-account-2fa space-y-3">
            {account?.["2fa_enabled"] ? (
              <TwoFactorEnabled />
            ) : (
              <TwoFactorDisabled />
            )}
            <p className="text-muted-foreground text-xs">
              {t("account_settings.looking_for_backup")}
            </p>
          </div>
        </div>

        {/* Password form */}
        <div className="rounded-xl border border-border/60 bg-card p-5 shadow-2xs space-y-4 md:col-span-2">
          <div>
            <h2 className="text-sm font-semibold tracking-tight text-foreground">
              {t("account.change_password_title")}
            </h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              定期更新密码以保证安全。
            </p>
          </div>
          <form onSubmit={changePassword} className="km-account-password-form max-w-lg space-y-3">
            <div className="space-y-1">
              <label htmlFor="password" className="text-xs font-medium text-foreground">{t("account.new_password")}</label>
              <TextField.Root
                className="w-full"
                id="password"
                name="password"
                type="password"
              />
            </div>
            <div className="space-y-1">
              <label htmlFor="password_repeat" className="text-xs font-medium text-foreground">
                {t("account.new_password_repeat")}
              </label>
              <TextField.Root
                className="w-full"
                id="password_repeat"
                name="password_repeat"
                type="password"
              />
            </div>
            {account?.["2fa_enabled"] ? (
              <div className="space-y-1">
                <label htmlFor="password_2fa" className="text-xs font-medium text-foreground">
                  {t("account.2fa_otp_input_prompt")}
                </label>
                <TextField.Root
                  className="w-full"
                  id="password_2fa"
                  name="password_2fa"
                  type="number"
                  placeholder="000000"
                  value={passwordTwoFa}
                  onChange={(e) =>
                    setPasswordTwoFa((e.target as HTMLInputElement).value)
                  }
                />
              </div>
            ) : null}
            <div className="pt-2">
              <Button disabled={passwordSaving} type="submit" variant="solid" className="cursor-pointer">
                {t("account.change_password_button")}
              </Button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
};
const TwoFactorDisabled = () => {
  const { t } = useTranslation();
  const { refresh } = useAccount();
  const [saving, setSaving] = React.useState(false);
  const [isOpen, setIsOpen] = React.useState(false);
  const [isLoading, setIsLoading] = React.useState(true);
  const [qrcode, setQRCode] = React.useState<string | null>(null);
  const [code, setCode] = React.useState<string>("");

  React.useEffect(() => {
    let objectUrl: string | null = null;
    let cancelled = false;
    if (isOpen) {
      setIsLoading(true);
      fetch("/api/admin/2fa/generate")
        .then((response) => {
          if (!response.ok) {
            throw new Error(t("account.qr_fetch_error"));
          }
          return response.blob();
        })
        .then((blob) => {
          if (cancelled) return;
          objectUrl = URL.createObjectURL(blob);
          setQRCode((previous) => {
            if (previous) URL.revokeObjectURL(previous);
            return objectUrl;
          });
        })
        .catch((err) => toast.error(err.message))
        .finally(() => setIsLoading(false));
    }
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [isOpen, t]);

  const handleEnable2fa = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!code) {
      toast.error(t("account.otp_empty_error"));
      return;
    }
    setSaving(true);
    fetch(`/api/admin/2fa/enable`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code }),
    })
      .then(async (res) => {
        if (!res.ok) {
          const data = await res.json();
          throw new Error(
            data.message || `Failed to enable 2FA (${res.status})`
          );
        }
        return res.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
        setIsOpen(false);
        refresh();
      })
      .catch((err) => toast.error(err.message))
      .finally(() => setSaving(false));
  };

  return (
    <Flex direction="column" gap="2" className="km-account-2fa-enable">
      <label className="text-lg font-bold">{t("account.2fa_disabled")}</label>
      <Dialog.Root open={isOpen} onOpenChange={setIsOpen}>
        <Dialog.Trigger>
          <div>
            <Button variant="solid" className="w-full cursor-pointer">{t("account.enable_2fa")}</Button>
          </div>
        </Dialog.Trigger>
        <Dialog.Content className="max-w-sm">
          <Dialog.Title>{t("account.enable_2fa")}</Dialog.Title>
          <div className="flex flex-col gap-3 my-1">
            <p className="text-xs text-muted-foreground text-center">
              {t("account.2fa_qr_code_hint")}
            </p>
            <div className="flex justify-center p-3 bg-white rounded-lg border border-border shadow-2xs mx-auto">
              {isLoading ? (
                <Skeleton width="180px" height="180px" />
              ) : (
                <img src={qrcode!} alt="2FA QR Code" width={180} height={180} className="rounded" />
              )}
            </div>
            <form className="km-account-2fa-form flex flex-col gap-3 mt-1" onSubmit={handleEnable2fa}>
              <div>
                <label className="text-xs font-medium text-muted-foreground block mb-1">
                  {t("account.2fa_otp_input_prompt")}
                </label>
                <TextField.Root
                  type="number"
                  name="code"
                  placeholder="000000"
                  className="text-center font-mono tracking-widest"
                  value={code}
                  onChange={(e) => setCode((e.target as HTMLInputElement).value)}
                  autoFocus
                />
              </div>
              <Flex gap="2" justify="end" mt="2">
                <Dialog.Close>
                  <Button variant="soft" color="gray" type="button" onClick={() => setIsOpen(false)}>
                    {t("common.cancel")}
                  </Button>
                </Dialog.Close>
                <Button disabled={saving || !code} type="submit">
                  {t("account.enable_2fa")}
                </Button>
              </Flex>
            </form>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </Flex>
  );
};

const TwoFactorEnabled = () => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [code, setCode] = React.useState("");
  const { refresh } = useAccount();
  const disable2fa = () => {
    if (!code) {
      toast.error(t("account.otp_empty_error"));
      return;
    }
    setSaving(true);
    fetch(`/api/admin/2fa/disable?2fa_code=${encodeURIComponent(code)}`, {
      method: "POST",
    })
      .then(async (response) => {
        if (!response.ok) {
          const data = await response.json();
          throw new Error(data.message || "Failed to disable 2FA");
        }
        return response.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
        setIsOpen(false);
        setCode("");
        refresh();
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setSaving(false);
      });
  };
  return (
    <Flex direction="column" gap="2" className="km-account-2fa-disable">
      <label>{t("account.2fa_enabled")}</label>
      <div>
        <Dialog.Root open={isOpen} onOpenChange={setIsOpen}>
          <Dialog.Trigger>
            <Button className="ml-2" color="red">
              {t("account.disable_2fa")}
            </Button>
          </Dialog.Trigger>
          <Dialog.Content className="max-w-sm">
            <Dialog.Title>{t("account.disable_2fa")}</Dialog.Title>
            <Dialog.Description>
              {t("account.disable_2fa_confirmation")}
            </Dialog.Description>
            <div className="flex flex-col gap-3 my-2">
              <div>
                <label htmlFor="disable_2fa_code" className="text-xs font-medium text-muted-foreground block mb-1">
                  {t("account.2fa_otp_input_prompt")}
                </label>
                <TextField.Root
                  id="disable_2fa_code"
                  type="number"
                  placeholder="000000"
                  className="font-mono text-center tracking-widest"
                  value={code}
                  onChange={(e) => setCode((e.target as HTMLInputElement).value)}
                  autoFocus
                />
              </div>
            </div>
            <Flex gap="2" justify="end" mt="4">
              <Dialog.Close>
                <Button variant="soft" color="gray" type="button" onClick={() => setIsOpen(false)}>
                  {t("common.cancel")}
                </Button>
              </Dialog.Close>
              <Button disabled={saving || !code} color="red" onClick={disable2fa}>
                {t("common.confirm")}
              </Button>
            </Flex>
          </Dialog.Content>
        </Dialog.Root>
      </div>
    </Flex>
  );
};

export default Account;
