import type { Metadata } from "next";
import Link from "next/link";
import { HashWithCopy } from "@/components/HashWithCopy";
import { RefreshButton } from "@/components/RefreshButton";
import { StatusBadge } from "@/components/StatusBadge";
import { TxLink } from "@/components/TxLink";
import { bytesToHex, formatTimestamp } from "@/lib/format";
import {
  getJobsPage,
  isPgBigint,
  JOB_STATUSES,
  type JobRow,
} from "@/lib/queries";

export const revalidate = 0;
export const metadata: Metadata = { title: "Jobs" };

const PER_PAGE = 50;

interface JobsSearchParams {
  status?: string;
  batch?: string;
  page?: string;
}

function jobsHref(params: { status?: string | null; batch?: string | null; page?: number }): string {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.batch) qs.set("batch", params.batch);
  if (params.page && params.page > 1) qs.set("page", String(params.page));
  const s = qs.toString();
  return s ? `/jobs?${s}` : "/jobs";
}

export default async function JobsPage({
  searchParams,
}: {
  searchParams: Promise<JobsSearchParams>;
}) {
  const sp = await searchParams;

  const status =
    sp.status && (JOB_STATUSES as readonly string[]).includes(sp.status)
      ? sp.status
      : null;
  const batchRaw = sp.batch?.trim() ?? "";
  const batchIsNumeric = /^[0-9]+$/.test(batchRaw);
  // Numeric but beyond int8 range: cannot exist in the DB, so treat the
  // search as matching nothing instead of letting Postgres throw on the cast.
  const batchOutOfRange = batchIsNumeric && !isPgBigint(batchRaw);
  const batch = batchIsNumeric && !batchOutOfRange ? batchRaw : null;
  const pageParam = Number(sp.page);
  const page = Number.isInteger(pageParam) && pageParam >= 1 ? pageParam : 1;

  const { jobs, total } = batchOutOfRange
    ? { jobs: [] as JobRow[], total: 0 }
    : await getJobsPage({
        status,
        batch,
        page,
        perPage: PER_PAGE,
      });
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));

  const abelExplorer = process.env.ABEL_EXPLORER_BASE || "https://explorer.pqabelian.io";
  const daExplorer = process.env.DA_EXPLORER_BASE || undefined;

  return (
    <>
      <div className="page-header">
        <h1 className="page-title">Jobs</h1>
        <div className="page-header-actions">
          <RefreshButton />
        </div>
      </div>

      <div className="filter-bar">
        <div className="filter-pills">
          <Link href={jobsHref({ batch })} className={`pill${status === null ? " active" : ""}`}>
            All
          </Link>
          {JOB_STATUSES.map((s) => (
            <Link
              key={s}
              href={jobsHref({ status: s, batch })}
              className={`pill${status === s ? " active" : ""}`}
            >
              {s}
            </Link>
          ))}
        </div>
        <form className="search-form" action="/jobs" method="get">
          {status ? <input type="hidden" name="status" value={status} /> : null}
          <input
            type="text"
            name="batch"
            inputMode="numeric"
            pattern="[0-9]*"
            placeholder="Batch number"
            defaultValue={batch ?? ""}
            aria-label="Search by batch number"
          />
          <button type="submit" className="btn">
            Search
          </button>
        </form>
      </div>

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
            {jobs.length === 0 ? (
              <tr>
                <td colSpan={6} className="empty-row">
                  No jobs match the current filter.
                </td>
              </tr>
            ) : (
              jobs.map((job) => (
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

      <div className="pagination">
        <span>
          {total} job{total === 1 ? "" : "s"} · page {page} of {totalPages}
        </span>
        <div className="pagination-links">
          <Link
            href={jobsHref({ status, batch, page: page - 1 })}
            className={`page-link${page <= 1 ? " disabled" : ""}`}
            aria-disabled={page <= 1}
          >
            Previous
          </Link>
          <Link
            href={jobsHref({ status, batch, page: page + 1 })}
            className={`page-link${page >= totalPages ? " disabled" : ""}`}
            aria-disabled={page >= totalPages}
          >
            Next
          </Link>
        </div>
      </div>
    </>
  );
}
