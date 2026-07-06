"use client";

import { useEffect } from "react";

export default function ProtectedError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Server-side details are already logged by Next; this is for the
    // browser console only.
    console.error(error);
  }, [error]);

  return (
    <div className="error-state">
      <h1 className="page-title">Something went wrong</h1>
      <p className="muted">
        The database may be unavailable, or an unexpected error occurred while
        loading this page. Your session is unaffected.
      </p>
      {error.digest ? (
        <p className="mono muted error-digest">Reference: {error.digest}</p>
      ) : null}
      <button type="button" className="btn btn-primary" onClick={() => reset()}>
        Retry
      </button>
    </div>
  );
}
