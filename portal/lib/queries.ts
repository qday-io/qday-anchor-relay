import { query } from "./db";

// pg returns BIGINT columns as strings, BYTEA as Buffer, JSONB as objects.

export const JOB_STATUSES = [
  "PENDING",
  "ABEL_SUBMITTED",
  "ABEL_CONFIRMED",
  "CONTRACT_RECORDED",
  "DONE",
  "FAILED",
] as const;
export type JobStatus = (typeof JOB_STATUSES)[number];

export const COIN_STATUSES = ["UNSPENT", "LOCKED", "SPENT"] as const;

export interface JobRow {
  id: string;
  batch_number: string;
  state_root: Buffer | null;
  da_block_number: string | null;
  da_tx_hash: string | null;
  abel_tx_hash: string | null;
  abel_height: string | null;
  record_tx_hash: string | null;
  status: string;
  retry_count: number;
  last_error: string | null;
  created_at: Date;
  updated_at: Date;
}

export interface JobEventRow {
  id: string;
  job_id: string;
  from_status: string | null;
  to_status: string;
  detail: unknown;
  created_at: Date;
}

export interface CoinRow {
  coin_id: string;
  tx_hash: string;
  out_index: number;
  value_neutrino: string;
  block_height: string | null;
  ring_id: string | null;
  ring_index: number | null;
  status: string;
  spent_by_job: string | null;
  created_at: Date;
  updated_at: Date;
}

const JOB_COLUMNS = `id, batch_number, state_root, da_block_number, da_tx_hash,
  abel_tx_hash, abel_height, record_tx_hash, status, retry_count, last_error,
  created_at, updated_at`;

export async function getJobCountsByStatus(): Promise<Record<string, number>> {
  const rows = await query<{ status: string; count: string }>(
    `SELECT status, count(*) AS count FROM anchor_jobs GROUP BY status`,
  );
  const counts: Record<string, number> = {};
  for (const row of rows) counts[row.status] = Number(row.count);
  return counts;
}

export async function getLatestJobs(limit: number): Promise<JobRow[]> {
  return query<JobRow>(
    `SELECT ${JOB_COLUMNS} FROM anchor_jobs ORDER BY id DESC LIMIT $1`,
    [limit],
  );
}

export interface JobsFilter {
  status: string | null;
  batch: string | null; // decimal string
  page: number; // 1-based
  perPage: number;
}

export async function getJobsPage(
  filter: JobsFilter,
): Promise<{ jobs: JobRow[]; total: number }> {
  const offset = (filter.page - 1) * filter.perPage;
  const [jobs, countRows] = await Promise.all([
    query<JobRow>(
      `SELECT ${JOB_COLUMNS} FROM anchor_jobs
       WHERE ($1::text IS NULL OR status = $1)
         AND ($2::bigint IS NULL OR batch_number = $2)
       ORDER BY id DESC
       LIMIT $3 OFFSET $4`,
      [filter.status, filter.batch, filter.perPage, offset],
    ),
    query<{ count: string }>(
      `SELECT count(*) AS count FROM anchor_jobs
       WHERE ($1::text IS NULL OR status = $1)
         AND ($2::bigint IS NULL OR batch_number = $2)`,
      [filter.status, filter.batch],
    ),
  ]);
  return { jobs, total: Number(countRows[0]?.count ?? 0) };
}

export async function getJobById(id: string): Promise<JobRow | null> {
  const rows = await query<JobRow>(
    `SELECT ${JOB_COLUMNS} FROM anchor_jobs WHERE id = $1`,
    [id],
  );
  return rows[0] ?? null;
}

export async function getJobEvents(jobId: string): Promise<JobEventRow[]> {
  return query<JobEventRow>(
    `SELECT id, job_id, from_status, to_status, detail, created_at
     FROM job_events WHERE job_id = $1
     ORDER BY created_at ASC, id ASC`,
    [jobId],
  );
}

export async function getCoins(): Promise<CoinRow[]> {
  return query<CoinRow>(
    `SELECT coin_id, tx_hash, out_index, value_neutrino, block_height, ring_id,
       ring_index, status, spent_by_job, created_at, updated_at
     FROM relay_coins
     ORDER BY block_height DESC NULLS LAST, coin_id ASC`,
  );
}

export interface RelayHealth {
  pollerCursor: string | null;
  abelCursor: string | null;
  unspentCount: number;
  unspentTotalNeutrino: string;
}

export async function getRelayHealth(): Promise<RelayHealth> {
  const [poller, abel, unspent] = await Promise.all([
    query<{ last_scanned_block: string }>(
      `SELECT last_scanned_block FROM poller_cursor WHERE id = 1`,
    ),
    query<{ last_scanned_height: string }>(
      `SELECT last_scanned_height FROM abel_cursor WHERE id = 1`,
    ),
    query<{ count: string; total: string }>(
      `SELECT count(*) AS count, COALESCE(sum(value_neutrino), 0)::text AS total
       FROM relay_coins WHERE status = 'UNSPENT'`,
    ),
  ]);
  return {
    pollerCursor: poller[0]?.last_scanned_block ?? null,
    abelCursor: abel[0]?.last_scanned_height ?? null,
    unspentCount: Number(unspent[0]?.count ?? 0),
    unspentTotalNeutrino: unspent[0]?.total ?? "0",
  };
}
