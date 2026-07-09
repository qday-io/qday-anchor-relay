import type { Metadata } from "next";
import { HashWithCopy } from "@/components/HashWithCopy";
import { RefreshButton } from "@/components/RefreshButton";
import { groupDigits, neutrinoToAbel } from "@/lib/format";
import { getUnspentSummary } from "@/lib/queries";
import {
  fetchWalletAddress,
  privacyLevelLabel,
  walletSourceLabel,
  type WalletAddress,
} from "@/lib/relay-admin";
import { GenerateAccount } from "./generate-account";

export const revalidate = 0;
export const metadata: Metadata = { title: "帳號設定" };

export default async function AccountsPage() {
  // Balance comes from Postgres and renders even when the relay admin API
  // is unreachable; the wallet metadata degrades to an inline error.
  const [wallet, balance] = await Promise.all([
    fetchWalletAddress().then(
      (result): WalletAddress | null => result,
      () => null, // relay down: render the page with inline errors instead
    ),
    getUnspentSummary(),
  ]);

  const relayDownError = (
    <div className="inline-error" role="alert">
      無法連線 relay admin API，暫時無法取得錢包資訊。請確認 relay
      服務狀態後點右上角 Refresh 重試。
    </div>
  );

  return (
    <>
      <div className="page-header">
        <h1 className="page-title">帳號設定</h1>
        <div className="page-header-actions">
          <RefreshButton />
        </div>
      </div>

      <h2 className="section-title" style={{ marginTop: 0 }}>
        發交易的帳號（Relay 錢包）
      </h2>
      <div className="detail-card">
        <table className="detail">
          <tbody>
            {wallet ? (
              <>
                <tr>
                  <th>網路</th>
                  <td>
                    <span className="badge badge-purple">{wallet.network}</span>
                  </td>
                </tr>
                <tr>
                  <th>隱私模式</th>
                  <td>{privacyLevelLabel(wallet.privacy_level)}</td>
                </tr>
                <tr>
                  <th>錢包來源</th>
                  <td>{walletSourceLabel(wallet.wallet_source)}</td>
                </tr>
              </>
            ) : (
              <tr>
                <th>錢包資訊</th>
                <td>{relayDownError}</td>
              </tr>
            )}
            <tr>
              <th>可用餘額</th>
              <td>
                <span className="mono">
                  {neutrinoToAbel(balance.totalNeutrino)} ABEL
                </span>{" "}
                <span className="muted mono">
                  （{groupDigits(balance.totalNeutrino)} Neutrino ·{" "}
                  {balance.count} 枚 UNSPENT coin）
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p className="muted address-note">
        Relay 以此錢包簽署並送出 Abelian 錨定交易；可用餘額為目前狀態是 UNSPENT
        的 relay coin 總和。
      </p>

      <h2 className="section-title">要轉帳過去的帳號（注資地址）</h2>
      {wallet ? (
        <div className="card">
          <div className="address-row">
            <HashWithCopy value={wallet.address_hex} head={18} tail={14} />
            <span className="muted mono address-net">{wallet.network}</span>
          </div>
          <p className="muted address-note">
            把測試 ABEL 轉到這個地址，就是幫 relay 錢包注資。這是同一個 relay
            錢包的收款地址：每次載入頁面顯示的地址不同，但都屬於同一個 relay
            錢包，皆可正常收款。
          </p>
          {wallet.note ? <p className="muted address-note">{wallet.note}</p> : null}
        </div>
      ) : (
        relayDownError
      )}

      <h2 className="section-title">建立測試帳號</h2>
      <GenerateAccount />
    </>
  );
}
