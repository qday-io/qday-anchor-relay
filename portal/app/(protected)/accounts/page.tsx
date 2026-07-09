import type { Metadata } from "next";
import { HashWithCopy } from "@/components/HashWithCopy";
import { RefreshButton } from "@/components/RefreshButton";
import { fetchWalletAddress, type WalletAddress } from "@/lib/relay-admin";
import { GenerateAccount } from "./generate-account";

export const revalidate = 0;
export const metadata: Metadata = { title: "測試帳號" };

export default async function AccountsPage() {
  let wallet: WalletAddress | null = null;
  try {
    wallet = await fetchWalletAddress();
  } catch {
    wallet = null; // relay down: render the page with an inline error instead
  }

  return (
    <>
      <div className="page-header">
        <h1 className="page-title">測試帳號</h1>
        <div className="page-header-actions">
          <RefreshButton />
        </div>
      </div>

      <h2 className="section-title" style={{ marginTop: 0 }}>
        Relay 錢包收款地址
      </h2>
      {wallet ? (
        <div className="card">
          <div className="address-row">
            <HashWithCopy value={wallet.address_hex} head={18} tail={14} />
            <span className="muted mono address-net">{wallet.network}</span>
          </div>
          <p className="muted address-note">
            向此地址轉入 ABEL 即可為 relay 錢包注資。每次載入頁面顯示的地址不同，
            但都屬於同一個 relay 錢包，皆可正常收款。
          </p>
          {wallet.note ? <p className="muted address-note">{wallet.note}</p> : null}
        </div>
      ) : (
        <div className="inline-error" role="alert">
          無法連線 relay admin API，暫時無法取得收款地址。請確認 relay
          服務狀態後點右上角 Refresh 重試。
        </div>
      )}

      <h2 className="section-title">產生測試 Abelian 帳號</h2>
      <GenerateAccount />
    </>
  );
}
