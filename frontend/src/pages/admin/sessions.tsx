import React from "react";
import { toast } from "sonner";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { useTranslation } from "react-i18next";
import { Dialog, Flex, Button } from "@radix-ui/themes";
import { UserAgentHelper } from "@/utils/UserAgentHelper";
import Loading from "@/components/loading";
type Resp = {
  current: string;
  data: Array<{
    uuid: string;
    id: string;
    user_agent: string;
    ip: string;
    login_method: string;
    latest_online: string;
    latest_ip: string;
    latest_user_agent: string;
    expires: string;
    created_at: string;
  }>;
  status: string;
};
export default function Sessions() {
  const [t] = useTranslation();
  const [sessions, setSessions] = React.useState<Resp | null>(null);
  React.useEffect(() => {
    fetch("/api/admin/session/get")
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Error: ${response.status} ${response.statusText}`);
        }
        return response.json();
      })
      .then((data: Resp) => {
        setSessions(data);
      })
      .catch((error) => {
        console.error("Error fetching sessions:", error);
        toast.error(error.message);
      });
  }, []);

  function deleteSession(sessionId: string) {
    const isCurrent = sessionId === sessions?.current;
    fetch("/api/admin/session/remove", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id: sessionId }),
    })
      .then((response) => response.json())
      .then((data) => {
        if (data.status === "success") {
          toast.success(t("sessions.deleted_successfully"));
          if (isCurrent) {
            window.location.href = "/"; // 登出
            return;
          }
          setSessions((prev) => ({
            ...prev!,
            data: prev?.data.filter((s) => s.id !== sessionId) || [],
          }));
        } else {
          console.error("Failed to delete session:", data);
          toast.error(t("sessions.delete_failed"));
        }
      })
      .catch((error) => {
        console.error("Error deleting session:", error);
        toast.error(error.message);
      });
  }
  function deleteAllSessions() {
    fetch("/api/admin/session/remove/all", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
    })
      .then((response) => {
        if (!response.ok) {
          toast.error("Error:" + response.status);
          return;
        }
        response
          .json()
          .then(() => {
            window.location.href = "/"; // 登出
          })
          .catch((error) => {
            toast.error("Error parsing JSON:" + error);
          });
      })
      .catch((error) => {
        toast.error(error.message);
      });
  }

  if (!sessions) {
    return <Loading />;
  }

  return (
    <div className="km-page-admin-sessions space-y-4">
      <div className="flex justify-between items-center">
        <h1 className="text-xl sm:text-2xl font-semibold tracking-tight text-foreground">
          {t("sessions.title")}
        </h1>
        <Dialog.Root>
          <Dialog.Trigger>
            <Button color="red" variant="soft" size="2">
              {t("sessions.delete_all")}
            </Button>
          </Dialog.Trigger>
          <Dialog.Content>
            <Dialog.Title>{t("sessions.delete_all")}</Dialog.Title>
            <Dialog.Description>
              {t("sessions.delete_all_desc")}
            </Dialog.Description>
            <Flex gap="2" justify="end" mt="4">
              <Dialog.Close>
                <Button variant="soft" color="gray">{t("common.cancel")}</Button>
              </Dialog.Close>
              <Button color="red" onClick={deleteAllSessions}>
                {t("common.delete")}
              </Button>
            </Flex>
          </Dialog.Content>
        </Dialog.Root>
      </div>
      <div className="km-sessions-table overflow-hidden rounded-lg border border-border bg-card shadow-2xs">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40 border-b border-border/80 text-[11px]">
              <TableHead className="font-mono">{t("sessions.session_id")}</TableHead>
              <TableHead>UA</TableHead>
              <TableHead className="font-mono">IP</TableHead>
              <TableHead className="font-mono">Latest IP</TableHead>
              <TableHead>{t("sessions.expires_at")}</TableHead>
              <TableHead>{t("sessions.last_login")}</TableHead>
              <TableHead className="text-right pr-4">{t("sessions.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions.data.map((s) => {
              const isCurrent = s.id === sessions.current;
              return (
                <TableRow key={s.uuid} className="km-session-item">
                  <TableCell>
                    <Dialog.Root>
                      <Dialog.Trigger>
                        <label className="hover:underline cursor-pointer">
                          {s.id.slice(0, 8)}...
                          {isCurrent && (
                            <span className="ml-2 text-[11px] font-mono px-2 py-0.5 rounded-full bg-foreground text-background font-medium">
                              {t("sessions.current")}
                            </span>
                          )}
                        </label>
                      </Dialog.Trigger>
                      <Dialog.Content className="max-w-lg">
                        <Dialog.Title>
                          {t("sessions.active_sessions")}
                        </Dialog.Title>
                        <div className="flex flex-col gap-3 my-2 text-xs">
                          <div className="p-2.5 rounded-lg border border-border/70 bg-muted/30">
                            <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">
                              {t("sessions.session_id")}
                            </span>
                            <span className="font-mono text-foreground select-all break-all text-xs">
                              {s.id}
                            </span>
                          </div>

                          <div className="grid grid-cols-2 gap-2.5">
                            <div className="p-2 rounded-lg border border-border/50 bg-muted/20">
                              <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">
                                IP / {t("sessions.latest_ip")}
                              </span>
                              <span className="font-mono text-foreground text-xs">
                                {s.ip} / {s.latest_ip}
                              </span>
                            </div>
                            <div className="p-2 rounded-lg border border-border/50 bg-muted/20">
                              <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">
                                {t("sessions.login_method")}
                              </span>
                              <span className="text-foreground text-xs capitalize">
                                {s.login_method}
                              </span>
                            </div>
                          </div>

                          <div className="p-2.5 rounded-lg border border-border/50 bg-muted/20">
                            <span className="text-[11px] font-medium text-muted-foreground block mb-1">
                              User Agent
                            </span>
                            <div className="font-medium text-foreground text-xs mb-1">
                              {UserAgentHelper.format(s.user_agent, t)}
                            </div>
                            <div className="text-[11px] text-muted-foreground font-mono break-all leading-tight">
                              {s.user_agent}
                            </div>
                            {s.latest_user_agent !== s.user_agent && (
                              <div className="mt-2 pt-2 border-t border-border/40">
                                <span className="text-[11px] font-medium text-muted-foreground block mb-1">
                                  {t("sessions.last_user_agent")}
                                </span>
                                <div className="font-medium text-foreground text-xs mb-1">
                                  {UserAgentHelper.format(s.latest_user_agent, t)}
                                </div>
                                <div className="text-[11px] text-muted-foreground font-mono break-all leading-tight">
                                  {s.latest_user_agent}
                                </div>
                              </div>
                            )}
                          </div>

                          <div className="grid grid-cols-2 gap-2.5 text-[11px]">
                            <div>
                              <span className="text-muted-foreground block">
                                {t("sessions.created_at")}
                              </span>
                              <span className="text-foreground mt-0.5 block">
                                {new Date(s.created_at).toLocaleString()}
                              </span>
                            </div>
                            <div>
                              <span className="text-muted-foreground block">
                                {t("sessions.expires_at")}
                              </span>
                              <span className="text-foreground mt-0.5 block">
                                {new Date(s.expires).toLocaleString()}
                              </span>
                            </div>
                            <div className="col-span-2 pt-1.5 border-t border-border/40">
                              <span className="text-muted-foreground block">
                                {t("sessions.latest_online")}
                              </span>
                              <span className="text-foreground mt-0.5 block">
                                {new Date(s.latest_online).toLocaleString()}{" "}
                                ({formatDuration(Date.now() - new Date(s.latest_online).getTime(), t)})
                              </span>
                            </div>
                          </div>
                        </div>

                        <Flex justify="end" mt="3">
                          <Dialog.Close>
                            <Button variant="soft" color="gray">{t("common.close")}</Button>
                          </Dialog.Close>
                        </Flex>
                      </Dialog.Content>
                    </Dialog.Root>
                  </TableCell>
                  <TableCell>{UserAgentHelper.format(s.user_agent, t)}</TableCell>
                  <TableCell>{s.ip}</TableCell>
                  <TableCell>{s.latest_ip}</TableCell>
                  <TableCell>{new Date(s.expires).toLocaleString()}</TableCell>
                  <TableCell>
                    {new Date(s.latest_online).toLocaleString()}{" "}
                    ({formatDuration((Date.now() - new Date(s.latest_online).getTime()), t)})
                  </TableCell>
                  <TableCell>
                    <Dialog.Root>
                      {!isCurrent && (
                        <Dialog.Trigger>
                          <Button color="red" variant="ghost">
                            {t("common.delete")}
                          </Button>
                        </Dialog.Trigger>
                      )}
                      <Dialog.Content>
                        <Dialog.Title>
                          {t("common.confirm_delete")}
                        </Dialog.Title>
                        <Dialog.Description>
                          {t("sessions.delete_one_desc")}
                        </Dialog.Description>
                        <Flex gap="2" justify="end" mt="4">
                          <Dialog.Close>
                            <Button variant="soft" color="gray">
                              {t("common.cancel")}
                            </Button>
                          </Dialog.Close>
                          <Button
                            color="red"
                            onClick={() => deleteSession(s.id)}
                          >
                            {t("common.delete")}
                          </Button>
                        </Flex>
                      </Dialog.Content>
                    </Dialog.Root>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

function formatDuration(number: number,t:any ): string {
  const ms = Math.abs(number);
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (seconds < 60) {
    return t("just_now");
  }

  if (days > 0) {
    const remainingHours = hours % 24;
    if (remainingHours > 0) {
      return `${days}${t("nodeCard.time_day")}${remainingHours}${t("nodeCard.time_hour")}${t("time.ago")}`;
    }
    return `${days}${t("nodeCard.time_day")} ${t("time.ago")}`;
  }

  if (hours > 0) {
    const remainingMinutes = minutes % 60;
    if (remainingMinutes > 0) {
      return `${hours}${t("nodeCard.time_hour")}${remainingMinutes}${t("nodeCard.time_minute")}${t("time.ago")}`;
    }
    return `${hours}${t("nodeCard.time_hour")}${t("time.ago")}`;
  }

  const remainingSeconds = seconds % 60;
  return `${minutes}${t("nodeCard.time_minute")}${remainingSeconds}${t("nodeCard.time_second")}${t("time.ago")}`;
}