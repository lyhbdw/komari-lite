import * as React from "react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTranslation } from "react-i18next";
import { Settings } from "lucide-react";
import { AccountProvider } from "@/contexts/AccountContext";
import { useAccount } from "@/contexts/useAccount";
import { usePublicInfo } from "@/contexts/usePublicInfo";

type LoginDialogProps = {
  trigger?: React.ReactNode | string;
  autoOpen?: boolean;
  showSettings?: boolean;
  info?: string | React.ReactNode;
  onLoginSuccess?: () => void;
};

const InnerLayout = ({
  trigger,
  autoOpen = false,
  showSettings = true,
  info,
  onLoginSuccess,
}: LoginDialogProps) => {
    const { account, loading, error, refresh } = useAccount();
    const [t] = useTranslation();
    const [username, setUsername] = React.useState("");
    const [password, setPassword] = React.useState("");
    const [twoFac, setTwoFac] = React.useState("");
    const [errorMsg, setErrorMsg] = React.useState("");
    const [isLoading, setIsLoading] = React.useState(false);
    const [require2FA, setRequire2FA] = React.useState(false);
    const [open, setOpen] = React.useState(autoOpen);
    const fieldId = React.useId().replace(/:/g, "");
    const {publicInfo} = usePublicInfo();

    React.useEffect(() => {
      if (autoOpen) {
        setOpen(true);
      }
    }, [autoOpen]);
  // 是否启用密码登录
  const passwordLoginEnabled = !publicInfo?.disable_password_login;

  // Validate inputs (仅在启用密码登录时需要)
  const isFormValid = passwordLoginEnabled && username.trim() !== "" && password.trim() !== "";
    // Handle login
    const handleLogin = async () => {
      if (!isFormValid) {
        setErrorMsg("Username and password are required");
        return;
      }

      setErrorMsg("");
      setIsLoading(true);
      try {
        const res = await fetch("/api/login", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            username,
            password,
            ...(twoFac && !account?.["2fa_enabled"] ? { "2fa_code": twoFac } : {}),
          }),
        });
        const data = await res.json();
        if (res.status === 200) {
          refresh();
          if (typeof onLoginSuccess === "function") {
            onLoginSuccess();
            return
          }
          window.open("/admin/servers", "_self");
        } else {
          if (data.message === "2FA code is required") {
            setRequire2FA(true);
            return;
          }
          setErrorMsg(data.message || "Login failed");
        }
      } catch (err) {
        setErrorMsg("Network error");
        console.error(err);
      } finally {
        setIsLoading(false);
      }
    };

    if (loading) {
      return <Button disabled>{t("loading")}</Button>;
    }
    if (error || !account) {
      return (
        <Button disabled color="red">
          Error
        </Button>
      );
    }
    if (account.logged_in) {
      if (!showSettings) {
        return null;
      }
      return (
        <a href="/admin/servers" target="_blank" rel="noreferrer">
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            title={t("settings.title", "Settings")}
            aria-label={t("settings.title", "Settings")}
          >
            <Settings size={16} />
          </Button>
        </a>
      );
    }

    return (
      <Dialog.Root open={open} onOpenChange={setOpen}>
        <Dialog.Trigger asChild>
          {trigger ? (typeof trigger === 'string' ? <Button>{trigger}</Button> : trigger) : <Button>{t("login.title")}</Button>}
        </Dialog.Trigger>
        <Dialog.Content className="km-login-dialog max-w-[420px] p-6 sm:p-7 rounded-2xl border border-border bg-card shadow-2xl z-50">
          <div className="flex flex-col items-center text-center mb-6">
            <div className="w-10 h-10 rounded-xl bg-foreground text-background flex items-center justify-center font-bold text-lg mb-3 shadow-sm select-none">
              K
            </div>
            <Dialog.Title className="text-xl font-bold tracking-tight text-foreground m-0">
              {t("login.title")}
            </Dialog.Title>
            <Dialog.Description className="text-xs text-muted-foreground mt-1.5 m-0">
              {info || t("login.desc")}
            </Dialog.Description>
          </div>
          <form
            className="km-login-form space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              if (isFormValid && !isLoading) {
                handleLogin();
              }
            }}
          >
            <div className="space-y-3.5">
              {passwordLoginEnabled && (
                <>
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-foreground block">
                      {t("login.username")}
                    </label>
                    <Input
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      id={`login-username-${fieldId}`}
                      name="username"
                      autoComplete="username"
                      placeholder="admin"
                      disabled={isLoading}
                      autoFocus
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-foreground block">
                      {t("login.password")}
                    </label>
                    <Input
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      id={`login-password-${fieldId}`}
                      name="password"
                      type="password"
                      autoComplete="current-password"
                      placeholder={t("login.password_placeholder")}
                      disabled={isLoading}
                    />
                  </div>
                  {require2FA && (
                    <div className="space-y-1.5">
                      <label className="text-xs font-medium text-foreground block">
                        {t("login.two_factor")}
                      </label>
                      <input
                        className="w-full h-10 px-3.5 text-sm font-mono tracking-widest text-center rounded-lg border border-border bg-background text-foreground placeholder:text-muted-foreground/60 outline-none focus:border-foreground/60 focus:ring-1 focus:ring-foreground/20 transition-all"
                        value={twoFac}
                        onChange={(e) => setTwoFac(e.target.value)}
                        id={`login-2fa-code-${fieldId}`}
                        name="2fa_code"
                        type="text"
                        autoComplete="one-time-code"
                        inputMode="numeric"
                        placeholder="000000"
                        disabled={isLoading}
                      />
                    </div>
                  )}
                  {errorMsg && (
                    <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-600 dark:text-rose-400 text-xs font-medium text-center">
                      {errorMsg}
                    </div>
                  )}
                  <button
                    type="submit"
                    disabled={isLoading || !isFormValid}
                    className={`w-full h-10 mt-2 rounded-lg font-medium text-sm transition-all flex items-center justify-center shadow-sm ${
                      isLoading || !isFormValid
                        ? "bg-muted text-muted-foreground cursor-not-allowed opacity-60"
                        : "bg-foreground text-background hover:opacity-90 active:scale-[0.99] cursor-pointer"
                    }`}
                  >
                    {isLoading ? "Logging in..." : t("login.title")}
                  </button>
                </>
              )}
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Root>
    );
};

const LoginDialog = (props: LoginDialogProps) => {
  return (
    <AccountProvider>
      <InnerLayout {...props} />
    </AccountProvider>
  );
};

export default LoginDialog;
