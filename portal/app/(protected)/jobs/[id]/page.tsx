import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { CopyButton } from "@/components/CopyButton";
import { HashWithCopy } from "@/components/HashWithCopy";
import { RefreshButton } from "@/components/RefreshButton";
import { StatusBadge } from "@/components/StatusBadge";
import { TxLink } from "@/components/TxLink";
import { bytesToHex, formatTimestamp, groupDigits } from "@/lib/format";
import { encodeMemo } from "@/lib/memo";
import { getJobById, getJobEvents, isPgBigint } from "@/lib/queries";

export const revalidate = 0;
export const metadata: Metadata = { title: "Job detail" };

function eventDotClass(toStatus: string): string {
  if (toStatus === "DONE") return " event-done";
  if (toStatus === "FAILED") return " event-failed";
  if (toStatus.startsWith("ABEL_")) return " event-abel";
  return "";
}

export default async function JobDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  // Reject non-numeric and out-of-int8-range ids before querying: a value
  // above 2^63-1 cannot exist and would make the Postgres int8 cast throw.
  if (!isPgBigint(id)) notFound();

  const job = await getJobById(id);
  if (!job) notFound();
  const events = await getJobEvents(id);

  const abelExplorer = process.env.ABEL_EXPLORER_BASE || "https://explorer.pqabelian.io";
  const daExplorer = process.env.DA_EXPLORER_BASE || undefined;

  const stateRootHex = bytesToHex(job.state_root);
  let memo: string | null = null;
  if (stateRootHex) {
    try {
      memo = encodeMemo(BigInt(job.batch_number), stateRootHex);
    } catch {
      memo = null; // malformed root in DB; show nothing rather than crash
    }
  }

  return (
    <>
      <div className="page-header">
        <h1 className="page-title">
          Job <span className="mono">#{job.id}</span>{" "}
          <StatusBadge status={job.status} />
        </h1>
        <div className="page-header-actions">
          <Link href="/jobs" className="btn btn-quiet" style={{ textDecoration: "none" }}>
            Back to jobs
          </Link>
          <RefreshButton />
        </div>
      </div>

      <h2 className="section-title" style={{ marginTop: 0 }}>
        Overview
      </h2>
      <div className="detail-card">
        <table className="detail">
          <tbody>
            <tr>
              <th>Batch number</th>
              <td className="mono">{job.batch_number}</td>
            </tr>
            <tr>
              <th>State root</th>
              <td>
                <HashWithCopy value={stateRootHex} full />
              </td>
            </tr>
            <tr>
              <th>DA block number</th>
              <td className="mono">
                {job.da_block_number !== null ? groupDigits(job.da_block_number) : "—"}
              </td>
            </tr>
            <tr>
              <th>DA tx hash</th>
              <td>
                <TxLink hash={job.da_tx_hash} explorerBase={daExplorer} kind="da" />
                {job.da_tx_hash ? <CopyButton value={job.da_tx_hash} /> : null}
              </td>
            </tr>
            <tr>
              <th>Abel tx hash</th>
              <td>
                <TxLink hash={job.abel_tx_hash} explorerBase={abelExplorer} kind="abel" />
                {job.abel_tx_hash ? <CopyButton value={job.abel_tx_hash} /> : null}
              </td>
            </tr>
            <tr>
              <th>Abel height</th>
              <td className="mono">
                {job.abel_height !== null ? groupDigits(job.abel_height) : "—"}
              </td>
            </tr>
            <tr>
              <th>Record tx hash</th>
              <td>
                <TxLink hash={job.record_tx_hash} explorerBase={daExplorer} kind="da" />
                {job.record_tx_hash ? <CopyButton value={job.record_tx_hash} /> : null}
              </td>
            </tr>
            <tr>
              <th>Retry count</th>
              <td className="mono">{job.retry_count}</td>
            </tr>
            <tr>
              <th>Last error</th>
              <td>
                {job.last_error ? (
                  <span className="mono error-text break-all">{job.last_error}</span>
                ) : (
                  <span className="muted">—</span>
                )}
              </td>
            </tr>
            <tr>
              <th>Created</th>
              <td className="mono">{formatTimestamp(job.created_at)}</td>
            </tr>
            <tr>
              <th>Updated</th>
              <td className="mono">{formatTimestamp(job.updated_at)}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <h2 className="section-title">Abelian memo</h2>
      {memo ? (
        <div className="memo-block">
          <code>{memo}</code>
          <CopyButton value={memo} />
        </div>
      ) : (
        <p className="muted">Memo unavailable (no state root recorded).</p>
      )}

      <h2 className="section-title">Event timeline</h2>
      {events.length === 0 ? (
        <p className="muted">No events recorded for this job.</p>
      ) : (
        <ul className="timeline">
          {events.map((event) => (
            <li key={event.id} className={eventDotClass(event.to_status)}>
              <div className="timeline-head">
                {event.from_status ? (
                  <>
                    <StatusBadge status={event.from_status} />
                    <span className="timeline-arrow">→</span>
                  </>
                ) : null}
                <StatusBadge status={event.to_status} />
                <span className="timeline-time">{formatTimestamp(event.created_at)}</span>
              </div>
              {event.detail != null ? (
                <pre className="timeline-detail">
                  {JSON.stringify(event.detail, null, 2)}
                </pre>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
