import { truncateMiddle } from "@/lib/format";
import { CopyButton } from "./CopyButton";

/** Middle-truncated monospace value with a copy-full-value button. */
export function HashWithCopy({
  value,
  head = 10,
  tail = 8,
  full = false,
}: {
  value: string | null;
  head?: number;
  tail?: number;
  /** Render the full value instead of truncating (detail pages). */
  full?: boolean;
}) {
  if (!value) return <span className="muted">—</span>;
  return (
    <span className="hash-cell">
      <span className={`mono${full ? " break-all" : ""}`} title={value}>
        {full ? value : truncateMiddle(value, head, tail)}
      </span>
      <CopyButton value={value} />
    </span>
  );
}
