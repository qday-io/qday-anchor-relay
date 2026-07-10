"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";

export function RefreshButton() {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  return (
    <>
      <span className="muted auto-refresh-note">每 10 秒自動更新</span>
      <button
        type="button"
        className="btn"
        disabled={isPending}
        onClick={() => startTransition(() => router.refresh())}
      >
        {isPending ? "Refreshing…" : "Refresh"}
      </button>
    </>
  );
}
