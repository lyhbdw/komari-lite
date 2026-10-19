import { toast } from "sonner";
import { copyTextToClipboard } from "../../../theme/src/utils/unitCore";

export async function copyToClipboard(
  text: string,
  options?: {
    successMessage?: string;
    errorMessage?: string;
  }
): Promise<boolean> {
  if (!text) return false;
  const ok = await copyTextToClipboard(text);
  if (ok) {
    if (options?.successMessage) {
      toast.success(options.successMessage);
    }
    return true;
  }
  toast.error(options?.errorMessage ?? "复制失败，请手动复制");
  return false;
}
