import type { Metadata } from "next";
import Link from "next/link";
import { HashWithCopy } from "@/components/HashWithCopy";
import { RefreshButton } from "@/components/RefreshButton";
import { StatusBadge } from "@/components/StatusBadge";
import { groupDigits, neutrinoToAbel } from "@/lib/format";
import { getCoins } from "@/lib/queries";

export const revalidate = 0;
export const metadata: Metadata = { title: "Coins" };

export default async function CoinsPage() {
  const coins = await getCoins();

  return (
    <>
      <div className="page-header">
        <h1 className="page-title">Relay coins</h1>
        <div className="page-header-actions">
          <RefreshButton />
        </div>
      </div>

      <div className="table-wrap">
        <table className="data">
          <thead>
            <tr>
              <th>Coin ID</th>
              <th className="num">Value (ABEL)</th>
              <th className="num">Value (Neutrino)</th>
              <th>Status</th>
              <th className="num">Block height</th>
              <th>Spent by job</th>
            </tr>
          </thead>
          <tbody>
            {coins.length === 0 ? (
              <tr>
                <td colSpan={6} className="empty-row">
                  No relay coins tracked yet.
                </td>
              </tr>
            ) : (
              coins.map((coin) => (
                <tr key={coin.coin_id}>
                  <td>
                    <HashWithCopy value={coin.coin_id} head={12} tail={10} />
                  </td>
                  <td className="num">{neutrinoToAbel(coin.value_neutrino)}</td>
                  <td className="num">{groupDigits(coin.value_neutrino)}</td>
                  <td>
                    <StatusBadge status={coin.status} />
                  </td>
                  <td className="num">
                    {coin.block_height !== null ? groupDigits(coin.block_height) : "—"}
                  </td>
                  <td>
                    {coin.spent_by_job !== null ? (
                      <Link href={`/jobs/${coin.spent_by_job}`} className="row-link mono">
                        #{coin.spent_by_job}
                      </Link>
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}
