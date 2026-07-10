"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

const REFRESH_INTERVAL_MS = 10_000;

/**
 * Re-fetches server component data every 10 seconds via router.refresh().
 * Mounted once in the (protected) layout. Ticks are skipped while the tab is
 * hidden (no pointless DB queries from background tabs); becoming visible
 * again triggers an immediate refresh. router.refresh() re-renders server
 * components only, so client state (e.g. a generated mnemonic) is preserved.
 */
export function AutoRefresh() {
  const router = useRouter();

  useEffect(() => {
    const intervalId = setInterval(() => {
      if (document.visibilityState === "visible") {
        router.refresh();
      }
    }, REFRESH_INTERVAL_MS);

    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        router.refresh();
      }
    };
    document.addEventListener("visibilitychange", onVisibilityChange);

    return () => {
      clearInterval(intervalId);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, [router]);

  return null;
}
