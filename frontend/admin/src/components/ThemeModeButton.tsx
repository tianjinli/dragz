import { Moon, Sun } from "lucide-react";
import type { MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { effectiveDark, usePreferencesStore } from "@/stores/preferences";
import {
  applyPreferencesThemeNow,
  runThemeViewTransition,
} from "@/utils/theme-transition";

export function ThemeModeButton({
  className = "",
  iconSize = 16,
}: {
  className?: string;
  iconSize?: number;
}) {
  const prefs = usePreferencesStore((state) => state.preferences);
  const update = usePreferencesStore((state) => state.update);
  const { t } = useTranslation();
  const dark = effectiveDark(prefs);

  const onToggle = (event: MouseEvent<HTMLButtonElement>) => {
    runThemeViewTransition(event, () => {
      const nextMode = dark ? ("light" as const) : ("dark" as const);
      const next = { ...prefs, theme: { ...prefs.theme, mode: nextMode } };
      applyPreferencesThemeNow(next);
      update({ theme: next.theme });
    });
  };

  return (
    <button
      aria-label={dark ? t("layout.themeToLight") : t("layout.themeToDark")}
      aria-live="polite"
      className={`theme-mode-button auth-icon-button is-${dark ? "dark" : "light"} ${className}`.trim()}
      type="button"
      onClick={onToggle}
    >
      {dark ? (
        <Sun className="theme-mode-icon" size={iconSize} />
      ) : (
        <Moon className="theme-mode-icon" fill="currentColor" size={iconSize} />
      )}
    </button>
  );
}
