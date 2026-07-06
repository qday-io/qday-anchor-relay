import { Pool, type QueryResultRow } from "pg";

// The portal connects with a READ-ONLY role (PORTAL_DB_USER). All queries are
// parameterized; DB access happens exclusively in Server Components and route
// handlers at request time — never at build time.

declare global {
  // eslint-disable-next-line no-var
  var __portalPgPool: Pool | undefined;
}

export function getPool(): Pool {
  if (!globalThis.__portalPgPool) {
    globalThis.__portalPgPool = new Pool({
      host: process.env.POSTGRES_HOST || "localhost",
      port: Number(process.env.POSTGRES_PORT || "5432"),
      database: process.env.POSTGRES_DB || "anchor",
      user: process.env.PORTAL_DB_USER || "portal_ro",
      password: process.env.PORTAL_DB_PASSWORD,
      max: 5,
      idleTimeoutMillis: 30_000,
      connectionTimeoutMillis: 10_000,
    });
  }
  return globalThis.__portalPgPool;
}

export async function query<T extends QueryResultRow>(
  text: string,
  params: readonly unknown[] = [],
): Promise<T[]> {
  const result = await getPool().query<T>(text, params as unknown[]);
  return result.rows;
}
