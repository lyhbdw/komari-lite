export interface BatchResult {
  succeeded: string[];
  failed: { uuid: string; message: string }[];
}

export function remainingSelection(selected: string[], result: BatchResult): string[] {
  const succeeded = new Set(result.succeeded);
  return selected.filter(uuid => !succeeded.has(uuid));
}

export async function runBatchNodes(
  uuids: string[], action: "group" | "delete", group = "", fetcher: typeof fetch = fetch,
): Promise<BatchResult> {
  const results = await Promise.all(uuids.map(async uuid => {
    try {
      const response = await fetcher(`/api/admin/client/${encodeURIComponent(uuid)}/${action === "group" ? "edit" : "remove"}`, {
        method: "POST",
        ...(action === "group" ? { headers: { "Content-Type": "application/json" }, body: JSON.stringify({ group }) } : {}),
      });
      if (response.status === 204) {
        return { uuid, ok: true as const };
      }
      const text = await response.text();
      let data;
      try { data = text ? JSON.parse(text) : null; }
      catch { throw new Error(response.ok ? "无效的服务器响应" : `HTTP ${response.status}`); }
      if (!data) {
        throw new Error(response.ok ? "无效的服务器响应" : `HTTP ${response.status}`);
      }
      const businessError = data.error || data.status === "error" || data.status === "failed" || data.success === false;
      if (!response.ok || businessError) {
        const message = data.error?.message || data.message || (typeof data.error === "string" ? data.error : null);
        throw new Error(message || (response.ok ? "操作失败" : `HTTP ${response.status}`));
      }
      return { uuid, ok: true as const };
    } catch (error) {
      return { uuid, ok: false as const, message: error instanceof Error ? error.message : String(error) };
    }
  }));
  return {
    succeeded: results.filter(result => result.ok).map(result => result.uuid),
    failed: results.filter(result => !result.ok).map(result => ({ uuid: result.uuid, message: result.message })),
  };
}
