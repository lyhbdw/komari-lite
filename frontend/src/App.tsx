import React from "react";
import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import { ThemeContext, THEME_DEFAULTS } from "./contexts/ThemeContext";
import { useSystemTheme } from "./hooks/useSystemTheme";
import { useRoutes } from "react-router-dom";
import { routes } from "./routes";
import Loading from "./components/loading";
import { PublicInfoProvider } from "./contexts/PublicInfoContext";
import { Toaster } from "./components/ui/sonner";
import { RPC2Provider } from "./contexts/RPC2Context";

export default function App() {
  const appearance = THEME_DEFAULTS.appearance;
  const color = THEME_DEFAULTS.color;
  const resolvedAppearance = useSystemTheme(appearance);

  React.useEffect(() => {
    localStorage.removeItem("appearance");
    localStorage.removeItem("color");
    const isDark = resolvedAppearance === "dark";
    document.documentElement.classList.toggle("dark", isDark);
  }, [resolvedAppearance]);

  const themeContextValue = {
    appearance,
    setAppearance: () => {},
    color,
    setColor: () => {},
  };
  const routing = useRoutes(routes);

  return (
    <React.Suspense fallback={<Loading />}>
      <ThemeContext.Provider value={themeContextValue}>
        <Theme
          appearance={resolvedAppearance}
          accentColor={color}
          scaling="100%"
          className="theme-root"
          style={{
            backgroundColor: "transparent",
            minHeight: "100vh",
          }}
        >
          <RPC2Provider>
            <PublicInfoProvider>
              <Toaster theme={resolvedAppearance} />
              {routing}
            </PublicInfoProvider>
          </RPC2Provider>
        </Theme>
      </ThemeContext.Provider>
    </React.Suspense>
  );
}
