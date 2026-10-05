import React, { useState } from "react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { useAccount } from "@/contexts/useAccount";
import { Button, Dialog, Flex, Skeleton } from "@radix-ui/themes";
import Loading from "@/components/loading";
import { Eye, EyeOff, KeyRound, QrCode, Shield, ShieldCheck, User } from "lucide-react";

const Account = () => <InnerLayout />;

const InnerLayout = () => {
  const { t } = useTranslation();
  const { account, loading, error, refresh } = useAccount();

  // 用户名编辑状态
  const [username, setUsername] = useState("");
  const [usernameSaving, setUsernameSaving] = useState(false);

  // 密码修改表单状态
  const [password, setPassword] = useState("");
  const [passwordRepeat, setPasswordRepeat] = useState("");
  const [passwordTwoFa, setPasswordTwoFa] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showPasswordRepeat, setShowPasswordRepeat] = useState(false);
  const [passwordSaving, setPasswordSaving] = useState(false);

  // 2FA 弹窗状态
  const [isEnable2FaOpen, setIsEnable2FaOpen] = useState(false);
  const [isDisable2FaOpen, setIsDisable2FaOpen] = useState(false);
  const [twoFaLoading, setTwoFaLoading] = useState(false);
  const [qrcodeUrl, setQrcodeUrl] = useState<string | null>(null);
  const [otpCode, setOtpCode] = useState("");

  React.useEffect(() => {
    if (account?.username) {
      setUsername(account.username);
    }
  }, [account?.username]);

  // 打开开启 2FA 弹窗时加载二维码
  React.useEffect(() => {
    let objectUrl: string | null = null;
    let cancelled = false;
    if (isEnable2FaOpen) {
      setTwoFaLoading(true);
      setOtpCode("");
      fetch("/api/admin/2fa/generate")
        .then((response) => {
          if (!response.ok) {
            throw new Error(t("account.qr_fetch_error", "无法生成 2FA 二维码"));
          }
          return response.blob();
        })
        .then((blob) => {
          if (cancelled) return;
          objectUrl = URL.createObjectURL(blob);
          setQrcodeUrl((previous) => {
            if (previous) URL.revokeObjectURL(previous);
            return objectUrl;
          });
        })
        .catch((err) => toast.error(err.message))
        .finally(() => setTwoFaLoading(false));
    }
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [isEnable2FaOpen, t]);

  if (loading) {
    return <Loading />;
  }
  if (error) {
    return <div className="p-4 text-xs text-rose-500">Error: {error.message}</div>;
  }

  // 提交修改用户名
  const handleSaveUsername = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = username.trim();
    if (!trimmed) {
      toast.error("用户名不能为空");
      return;
    }
    if (trimmed.length < 3) {
      toast.error("用户名长度至少需要 3 个字符");
      return;
    }
    setUsernameSaving(true);
    try {
      const response = await fetch("/api/admin/update/user", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          uuid: account?.uuid,
          username: trimmed,
        }),
      });
      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.message || "更新用户名失败");
      }
      toast.success(t("common.updated_successfully", "更新成功"));
      refresh();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setUsernameSaving(false);
    }
  };

  // 提交修改密码
  const handleSavePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!password || !passwordRepeat) {
      toast.error(t("account.password_empty_error", "请输入新密码"));
      return;
    }
    if (password !== passwordRepeat) {
      toast.error(t("account.password_mismatch_error", "两次输入的密码不一致"));
      return;
    }
    if (password.length < 8) {
      toast.error(t("account.password_too_short_error", "密码长度至少需要 8 位"));
      return;
    }
    if (!/(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/.test(password)) {
      toast.error(t("account.password_strength_error", "密码必须包含大小写字母和数字"));
      return;
    }
    if (account?.["2fa_enabled"] && !passwordTwoFa) {
      toast.error(t("account.otp_empty_error", "请输入 6 位动态验证码"));
      return;
    }

    setPasswordSaving(true);
    try {
      const response = await fetch("/api/admin/update/user", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          uuid: account?.uuid,
          password,
          "2fa_code": passwordTwoFa,
        }),
      });
      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.message || "修改密码失败");
      }
      toast.success("密码更新成功，正在返回登录页...");
      setPassword("");
      setPasswordRepeat("");
      setPasswordTwoFa("");
      setTimeout(() => {
        window.location.href = "/";
      }, 1500);
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setPasswordSaving(false);
    }
  };

  // 确认启用 2FA
  const handleConfirmEnable2Fa = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanCode = otpCode.trim();
    if (!cleanCode) {
      toast.error(t("account.otp_empty_error", "请输入 6 位动态验证码"));
      return;
    }
    setTwoFaLoading(true);
    try {
      const res = await fetch("/api/admin/2fa/enable", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code: cleanCode }),
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.message || "2FA 激活校验失败");
      }
      toast.success(t("common.updated_successfully", "两步验证启用成功"));
      setIsEnable2FaOpen(false);
      setOtpCode("");
      refresh();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setTwoFaLoading(false);
    }
  };

  // 确认停用 2FA
  const handleConfirmDisable2Fa = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanCode = otpCode.trim();
    if (!cleanCode) {
      toast.error(t("account.otp_empty_error", "请输入 6 位动态验证码"));
      return;
    }
    setTwoFaLoading(true);
    try {
      const res = await fetch(`/api/admin/2fa/disable?2fa_code=${encodeURIComponent(cleanCode)}`, {
        method: "POST",
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.message || "停用两步验证失败");
      }
      toast.success(t("common.updated_successfully", "两步验证已停用"));
      setIsDisable2FaOpen(false);
      setOtpCode("");
      refresh();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setTwoFaLoading(false);
    }
  };

  const is2FaActive = !!account?.["2fa_enabled"];

  return (
    <div className="km-page-admin-account max-w-4xl space-y-4">
      {/* 1. 统一精致页头 */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("account.title", "管理员账户")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              {account?.username}
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            管理后台管理员凭证、两步验证（2FA）绑定与登录安全策略。
          </p>
        </div>
      </div>

      {/* 2. 账号与两步验证卡片 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center gap-2 pb-2 border-b border-border/40">
          <User size={16} className="text-muted-foreground" />
          <h2 className="text-sm font-semibold text-foreground">账户与两步验证</h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-1">
          {/* 左列：用户名修改 */}
          <form
            onSubmit={handleSaveUsername}
            className="flex flex-col justify-between p-3.5 rounded-lg border border-border/60 bg-muted/20"
          >
            <div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-medium text-foreground block">
                  {t("account.change_username_title", "登录用户名")}
                </span>
                <span className="px-1.5 py-0.2 text-[10px] font-mono rounded bg-muted text-muted-foreground border border-border">
                  UID #{account?.uuid ? account.uuid.slice(0, 8) : "-"}
                </span>
              </div>
              <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                用于登录管理后台的管理员主账号名称。
              </p>
            </div>
            <div className="mt-3 flex items-center gap-2">
              <input
                type="text"
                id="admin-username-input"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="例如：admin"
                className="w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
              />
              <button
                type="submit"
                disabled={usernameSaving || !username || username === account?.username}
                className="h-8 px-3 rounded-md bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs shrink-0 disabled:opacity-40"
              >
                {usernameSaving ? "保存中" : t("account.change_username_button", "保存")}
              </button>
            </div>
          </form>

          {/* 右列：2FA 两步验证 */}
          <div className="flex flex-col justify-between p-3.5 rounded-lg border border-border/60 bg-muted/20">
            <div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-medium text-foreground block">
                  两步验证 (2FA)
                </span>
                {is2FaActive ? (
                  <span className="inline-flex items-center gap-1 px-1.5 py-0.2 text-[10px] font-mono rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                    <ShieldCheck size={11} />
                    <span>已启用保护</span>
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 px-1.5 py-0.2 text-[10px] font-mono rounded bg-muted text-muted-foreground border border-border">
                    <Shield size={11} />
                    <span>未启用</span>
                  </span>
                )}
              </div>
              <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                登录时额外要求输入身份验证器动态口令，大幅提高安全性。
              </p>
            </div>

            <div className="mt-3 flex justify-end">
              {is2FaActive ? (
                <button
                  type="button"
                  onClick={() => {
                    setOtpCode("");
                    setIsDisable2FaOpen(true);
                  }}
                  className="h-8 px-3 rounded-md border border-rose-500/30 text-rose-600 dark:text-rose-400 bg-rose-500/10 hover:bg-rose-500/20 text-xs font-medium active:scale-[0.98] transition-all cursor-pointer shadow-2xs"
                >
                  {t("account.disable_2fa", "停用 2FA")}
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => {
                    setOtpCode("");
                    setIsEnable2FaOpen(true);
                  }}
                  className="h-8 px-3 rounded-md bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs inline-flex items-center gap-1.5"
                >
                  <QrCode size={13} />
                  <span>{t("account.enable_2fa", "开启两步验证")}</span>
                </button>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* 3. 修改密码卡片 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div className="flex items-center gap-2">
            <KeyRound size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">
              {t("account.change_password_title", "修改登录密码")}
            </h2>
          </div>
        </div>

        <form onSubmit={handleSavePassword} className="space-y-4 pt-1">
          <div className={`grid grid-cols-1 ${is2FaActive ? "md:grid-cols-3" : "md:grid-cols-2"} gap-4`}>
            {/* 新密码 */}
            <div className="space-y-1.5">
              <label htmlFor="new-password" className="text-xs font-medium text-foreground block">
                {t("account.new_password", "新密码")}
              </label>
              <div className="relative">
                <input
                  id="new-password"
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="至少 8 位，含大小写字母与数字"
                  className="w-full h-8 pl-2.5 pr-8 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-2 top-2 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                  title={showPassword ? "隐藏密码" : "显示明文"}
                >
                  {showPassword ? <EyeOff size={13} /> : <Eye size={13} />}
                </button>
              </div>
            </div>

            {/* 确认新密码 */}
            <div className="space-y-1.5">
              <label htmlFor="repeat-password" className="text-xs font-medium text-foreground block">
                {t("account.new_password_repeat", "确认新密码")}
              </label>
              <div className="relative">
                <input
                  id="repeat-password"
                  type={showPasswordRepeat ? "text" : "password"}
                  value={passwordRepeat}
                  onChange={(e) => setPasswordRepeat(e.target.value)}
                  placeholder="再次输入新密码"
                  className="w-full h-8 pl-2.5 pr-8 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
                />
                <button
                  type="button"
                  onClick={() => setShowPasswordRepeat(!showPasswordRepeat)}
                  className="absolute right-2 top-2 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                  title={showPasswordRepeat ? "隐藏密码" : "显示明文"}
                >
                  {showPasswordRepeat ? <EyeOff size={13} /> : <Eye size={13} />}
                </button>
              </div>
            </div>

            {/* 2FA 验证码（仅在已启用 2FA 时渲染） */}
            {is2FaActive && (
              <div className="space-y-1.5">
                <label htmlFor="2fa-auth-code" className="text-xs font-medium text-foreground block">
                  2FA 动态口令
                </label>
                <input
                  id="2fa-auth-code"
                  type="number"
                  value={passwordTwoFa}
                  onChange={(e) => setPasswordTwoFa(e.target.value)}
                  placeholder="6 位动态验证码"
                  className="w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono text-center tracking-widest"
                />
              </div>
            )}
          </div>

          <div className="flex items-center justify-between pt-2">
            <p className="text-[11px] text-muted-foreground">
              密码更新成功后，当前会话将安全注销并提示重新登录。
            </p>
            <button
              type="submit"
              disabled={passwordSaving || !password || !passwordRepeat}
              className="h-8 px-4 rounded-lg bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs disabled:opacity-40"
            >
              {passwordSaving ? "正在更新..." : t("account.change_password_button", "保存新密码")}
            </button>
          </div>
        </form>
      </div>

      {/* 4. 启用 2FA 弹窗 */}
      <Dialog.Root open={isEnable2FaOpen} onOpenChange={setIsEnable2FaOpen}>
        <Dialog.Content className="max-w-sm">
          <Dialog.Title>
            <div className="flex items-center gap-2">
              <QrCode size={16} className="text-muted-foreground" />
              <span>{t("account.enable_2fa", "启用两步验证 (2FA)")}</span>
            </div>
          </Dialog.Title>

          <form onSubmit={handleConfirmEnable2Fa} className="flex flex-col gap-3 my-2 text-xs">
            <p className="text-[11px] text-muted-foreground leading-relaxed text-center">
              {t("account.2fa_qr_code_hint", "使用 Google Authenticator、1Password 等身份验证应用扫描下方二维码：")}
            </p>

            <div className="flex justify-center p-3 bg-white rounded-lg border border-border/70 shadow-2xs mx-auto">
              {twoFaLoading ? (
                <Skeleton width="180px" height="180px" />
              ) : (
                <img
                  src={qrcodeUrl || ""}
                  alt="2FA 二维码"
                  width={180}
                  height={180}
                  className="rounded"
                />
              )}
            </div>

            <div className="space-y-1.5 pt-1">
              <label htmlFor="enable-otp-input" className="text-xs font-medium text-foreground block text-center">
                输入应用生成的 6 位动态验证码
              </label>
              <input
                id="enable-otp-input"
                type="number"
                value={otpCode}
                onChange={(e) => setOtpCode(e.target.value)}
                placeholder="000000"
                maxLength={6}
                autoFocus
                className="w-full h-9 px-3 text-center text-sm font-mono tracking-widest rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs"
              />
            </div>

            <Flex gap="2" justify="end" mt="3">
              <Dialog.Close>
                <Button variant="soft" color="gray" type="button" className="cursor-pointer">
                  {t("common.cancel", "取消")}
                </Button>
              </Dialog.Close>
              <Button
                type="submit"
                disabled={twoFaLoading || !otpCode.trim()}
                className="cursor-pointer"
              >
                {twoFaLoading ? "验证中..." : "确认绑定"}
              </Button>
            </Flex>
          </form>
        </Dialog.Content>
      </Dialog.Root>

      {/* 5. 停用 2FA 确认弹窗 */}
      <Dialog.Root open={isDisable2FaOpen} onOpenChange={setIsDisable2FaOpen}>
        <Dialog.Content className="max-w-sm">
          <Dialog.Title>
            <div className="flex items-center gap-2 text-rose-600 dark:text-rose-400">
              <Shield size={16} />
              <span>{t("account.disable_2fa", "停用两步验证")}</span>
            </div>
          </Dialog.Title>

          <form onSubmit={handleConfirmDisable2Fa} className="flex flex-col gap-3 my-2 text-xs">
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {t("account.disable_2fa_confirmation", "停用后，登录将仅依赖密码验证，安全性将降低。请输入当前 6 位验证码确认：")}
            </p>

            <div className="space-y-1.5">
              <input
                type="number"
                value={otpCode}
                onChange={(e) => setOtpCode(e.target.value)}
                placeholder="000000"
                maxLength={6}
                autoFocus
                className="w-full h-9 px-3 text-center text-sm font-mono tracking-widest rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs"
              />
            </div>

            <Flex gap="2" justify="end" mt="3">
              <Dialog.Close>
                <Button variant="soft" color="gray" type="button" className="cursor-pointer">
                  {t("common.cancel", "取消")}
                </Button>
              </Dialog.Close>
              <Button
                type="submit"
                color="red"
                disabled={twoFaLoading || !otpCode.trim()}
                className="cursor-pointer"
              >
                {twoFaLoading ? "处理中..." : "确认停用"}
              </Button>
            </Flex>
          </form>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  );
};

export default Account;
