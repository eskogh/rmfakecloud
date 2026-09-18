import { useEffect, useState } from "react";
export default function useTheme() {
  const [theme, setTheme] = useState(() => {
    try {
      return localStorage.getItem("rm-theme") || "system";
    } catch {
      return "system";
    }
  });
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      document.documentElement.dataset.bsTheme =
        theme === "system" ? (media.matches ? "dark" : "light") : theme;
    };
    apply();
    media.addEventListener("change", apply);
    try {
      localStorage.setItem("rm-theme", theme);
    } catch {
      /* Session-only preference. */
    }
    return () => media.removeEventListener("change", apply);
  }, [theme]);
  return [theme, setTheme];
}
