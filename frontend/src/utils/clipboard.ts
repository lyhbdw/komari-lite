import { useCallback, useState } from "react";
import { toast } from "sonner";

export async function copyToClipboard(
  text: string,
  options?: {
    successMessage?: string;
    errorMessage?: string;
  }
): Promise<boolean> {
  if (!text) return false;
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    if (options?.successMessage) {
      toast.success(options.successMessage);
    }
    return true;
  } catch {
    toast.error(options?.errorMessage ?? "复制失败，请手动复制");
    return false;
  }
}

export function useClipboard(timeout = 2000) {
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const copy = useCallback(
    async (text: string, id: string = text, successMessage?: string) => {
      const ok = await copyToClipboard(text, { successMessage });
      if (ok) {
        setCopiedId(id);
        setTimeout(() => setCopiedId(null), timeout);
      }
      return ok;
    },
    [timeout]
  );

  return { copiedId, isCopied: (id: string) => copiedId === id, copy };
}
