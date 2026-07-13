"use client";

import { useState } from "react";
import { CopyButton } from "@/components/CopyButton";
import { truncateMiddle } from "@/lib/format";
// Type-only import: erased at compile time, pulls no server code client-side.
import type { GeneratedAccount } from "@/lib/relay-admin";

export function GenerateAccount() {
  const [account, setAccount] = useState<GeneratedAccount | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function generate() {
    setPending(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/generate-account", {
        method: "POST",
        cache: "no-store",
      });
      const data = await res.json().catch(() => null);
      if (!res.ok) {
        setAccount(null);
        setError(
          (data && typeof data.error === "string" && data.error) ||
            "產生帳號失敗，請稍後再試。",
        );
        return;
      }
      setAccount(data as GeneratedAccount);
    } catch {
      setAccount(null);
      setError("網路錯誤，請稍後再試。");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="card">
      <p className="muted address-note" style={{ marginTop: 0 }}>
        產生一組全新的 Abelian 測試帳號（mnemonic 與收款地址）。
        這樣可以先自己建立測試帳號來測試轉帳。
      </p>
      <button
        type="button"
        className="btn btn-primary"
        onClick={generate}
        disabled={pending}
      >
        {pending ? "產生中…" : "產生新帳號"}
      </button>

      {error ? (
        <div className="inline-error" role="alert" style={{ marginTop: 14 }}>
          {error}
        </div>
      ) : null}

      {account ? (
        <div className="generated-account">
          <div className="callout-warning" role="alert">
            助記詞只顯示這一次，不會被儲存——請立即保存。測試網專用。
          </div>

          <div className="field-label">Mnemonic</div>
          <div className="memo-block">
            <code>{account.mnemonic}</code>
            <CopyButton value={account.mnemonic} />
          </div>

          {account.short_address_hex ? (
            <>
              <div className="field-label">短地址</div>
              <div className="address-row">
                <span className="hash-cell">
                  <span className="mono" title={account.short_address_hex}>
                    {truncateMiddle(account.short_address_hex, 18, 14)}
                  </span>
                  <CopyButton value={account.short_address_hex} />
                </span>
                <span className="muted mono address-net">{account.network}</span>
              </div>

              <div className="field-label">完整地址</div>
              <div className="address-row">
                <span className="hash-cell">
                  <span className="mono" title={account.address_hex}>
                    {truncateMiddle(account.address_hex, 18, 14)}
                  </span>
                  <CopyButton value={account.address_hex} />
                </span>
              </div>
            </>
          ) : (
            <>
              <div className="field-label">Address</div>
              <div className="address-row">
                <span className="hash-cell">
                  <span className="mono" title={account.address_hex}>
                    {truncateMiddle(account.address_hex, 18, 14)}
                  </span>
                  <CopyButton value={account.address_hex} />
                </span>
                <span className="muted mono address-net">{account.network}</span>
              </div>
            </>
          )}

          {account.ans_registered === true ? (
            <p className="address-note ans-ok">已註冊 ANS，錢包可直接解析</p>
          ) : account.ans_registered === false ? (
            <p className="address-note error-text">
              ANS 註冊失敗——錢包可能無法解析短地址，請改用完整地址
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
