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
