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
      document
        .querySelector('meta[name="theme-color"]')
        ?.setAttribute(
          "content",
          document.documentElement.dataset.bsTheme === "dark"
            ? "#0d0f14"
            : "#f5f3f9",
        );
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
