import { Flex } from "@radix-ui/themes";
import { Outlet } from "react-router-dom";

export default function SettingLayout() {
  return (
    <Flex direction="column" gap="3" className="km-admin-settings-layout km-admin-settings-content max-w-4xl">
      <Outlet />
    </Flex>
  );
}
