import { createContext } from 'react';

export type Colors =
  | "gray" | "gold" | "bronze" | "brown" | "yellow" | "amber"
  | "orange" | "tomato" | "red" | "ruby" | "crimson" | "pink"
  | "plum" | "purple" | "violet" | "iris" | "indigo" | "blue"
  | "cyan" | "teal" | "jade" | "green" | "grass" | "lime"
  | "mint" | "sky";

export type Appearance = "light" | "dark" | "system";

export const THEME_DEFAULTS = {
  appearance: "system" as Appearance,
  color: "gray" as Colors,
} as const;

export interface ThemeContextType {
  appearance: Appearance;
  setAppearance: (appearance: Appearance) => void;
  color: Colors;
  setColor: (color: Colors) => void;
}

export const ThemeContext = createContext<ThemeContextType>({
  appearance: THEME_DEFAULTS.appearance,
  setAppearance: () => {},
  color: THEME_DEFAULTS.color,
  setColor: () => {},
});

