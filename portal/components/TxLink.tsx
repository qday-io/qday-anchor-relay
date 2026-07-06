import { truncateMiddle } from "@/lib/format";

/**
 * Renders a transaction hash, truncated, linking to the block explorer when
 * a base URL is configured. `kind` "abel" gets the Abelian purple.
 */
export function TxLink({
  hash,
  explorerBase,
  kind,
}: {
  hash: string | null;
  explorerBase: string | undefined;
  kind: "abel" | "da";
}) {
  if (!hash) return <span className="muted">—</span>;

  const short = truncateMiddle(hash, 10, 8);
  const base = explorerBase?.trim().replace(/\/+$/, "");
  if (!base) {
    return (
      <span className="mono" title={hash}>
        {short}
      </span>
    );
  }
  return (
    <a
      className={`mono tx-link${kind === "abel" ? " link-abel" : ""}`}
      href={`${base}/tx/${hash}`}
      target="_blank"
      rel="noopener noreferrer"
      title={hash}
    >
      {short}
    </a>
  );
}
