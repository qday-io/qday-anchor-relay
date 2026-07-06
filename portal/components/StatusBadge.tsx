const BADGE_CLASS: Record<string, string> = {
  // Job statuses
  PENDING: "badge-gray",
  ABEL_SUBMITTED: "badge-purple",
  ABEL_CONFIRMED: "badge-purple",
  CONTRACT_RECORDED: "badge-gray",
  DONE: "badge-green",
  FAILED: "badge-amber",
  // Coin statuses
  UNSPENT: "badge-green",
  LOCKED: "badge-gray",
  SPENT: "badge-muted",
};

export function StatusBadge({ status }: { status: string }) {
  const cls = BADGE_CLASS[status] ?? "badge-gray";
  return <span className={`badge ${cls}`}>{status}</span>;
}
