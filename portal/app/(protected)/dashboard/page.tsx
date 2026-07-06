import type { Metadata } from "next";
import Link from "next/link";
import { HashWithCopy } from "@/components/HashWithCopy";
import { RefreshButton } from "@/components/RefreshButton";
import { StatusBadge } from "@/components/StatusBadge";
import { TxLink } from "@/components/TxLink";
import {
  bytesToHex,
  formatTimestamp,
  groupDigits,
  neutrinoToAbel,
} from "@/lib/format";
import {
  getJobCountsByStatus,
  getLatestJobs,
  getRelayHealth,
  JOB_STATUSES,
} from "@/lib/queries";

export const revalidate = 0;
export const metadata: Metadata = { title: "Dashboard" };

const CARD_ACCENT: Record<string, string> = {
  DONE: " card-accent-green",
  FAILED: " card-accent-amber",
  ABEL_SUBMITTED: " card-accent-purple",
  ABEL_CONFIRMED: " card-accent-purple",
};

export default async function DashboardPage() {
  const [counts, latestJobs, health] = await Promise.all([
    getJobCountsByStatus(),
    getLatestJobs(10),
    getRelayHealth(),
  ]);

  const abelExplorer = process.env.ABEL_EXPLORER_BASE || "https://explorer.pqabelian.io";
  const daExplorer = process.env.DA_EXPLORER_BASE || undefined;

  return (
    <>
      <div className="page-header">
        <h1 className="page-title">Dashboard</h1>
        <div className="page-header-actions">
          <RefreshButton />
        </div>
      </div>

      <h2 className="section-title" style={{ marginTop: 0 }}>
        Jobs by status
      </h2>
      <div className="card-grid">
        {JOB_STATUSES.map((status) => (
          <div key={status} className={`card${CARD_ACCENT[status] ?? ""}`}>
            <div className="card-label">{status.replace(/_/g, " ")}</div>
            <div className="card-value">{counts[status] ?? 0}</div>
          </div>
        ))}
      </div>

      <h2 className="section-title">Relay health</h2>
      <div className="card-grid">
        <div className="card">
          <div className="card-label">DA poller cursor</div>
          <div className="card-value">
            {health.pollerCursor !== null ? groupDigits(health.pollerCursor) : "—"}
          </div>
          <div className="card-sub">last scanned block</div>
        </div>
        <div className="card">
          <div className="card-label">Abelian cursor</div>
          <div className="card-value">
            {health.abelCursor !== null ? groupDigits(health.abelCursor) : "—"}
          </div>
          <div className="card-sub">last scanned height</div>
        </div>
        <div className="card">
          <div className="card-label">Unspent coins</div>
          <div className="card-value">{health.unspentCount}</div>
          <div className="card-sub">
            {neutrinoToAbel(health.unspentTotalNeutrino)} ABEL (
            {groupDigits(health.unspentTotalNeutrino)} Neutrino)
          </div>
        </div>
        <div className={`card${(counts.FAILED ?? 0) > 0 ? " card-accent-amber" : ""}`}>
          <div className="card-label">Failed jobs</div>
          <div className="card-value">{counts.FAILED ?? 0}</div>
        </div>
      </div>

      <h2 className="section-title">Latest jobs</h2>
      <div className="table-wrap">
        <table className="data">
          <thead>
            <tr>
              <th className="num">Batch #</th>
              <th>State root</th>
              <th>Status</th>
              <th>Abel tx</th>
              <th>Record tx</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            {latestJobs.length === 0 ? (
              <tr>
                <td colSpan={6} className="empty-row">
                  No jobs yet.
                </td>
              </tr>
            ) : (
              latestJobs.map((job) => (
                <tr key={job.id}>
                  <td className="num">
                    <Link href={`/jobs/${job.id}`} className="row-link mono">
                      {job.batch_number}
                    </Link>
                  </td>
                  <td>
                    <HashWithCopy value={bytesToHex(job.state_root)} />
                  </td>
                  <td>
                    <StatusBadge status={job.status} />
                  </td>
                  <td>
                    <TxLink hash={job.abel_tx_hash} explorerBase={abelExplorer} kind="abel" />
                  </td>
                  <td>
                    <TxLink hash={job.record_tx_hash} explorerBase={daExplorer} kind="da" />
                  </td>
                  <td className="mono muted">{formatTimestamp(job.updated_at)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}
