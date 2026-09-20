// The only place in the frontend that knows the Go API exists.
//
// It owns one non-obvious job: the backend token. The UI must never think
// about tokens, so this module caches one in memory, refreshes it before it
// expires, and retries exactly once on a 401 (which happens when a token
// expires mid-request or the server restarts).

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

let cached: { token: string; expiresAt: number } | null = null;
let inFlight: Promise<string> | null = null;

async function mintToken(): Promise<string> {
  const res = await fetch("/api/token", { cache: "no-store" });
  if (!res.ok) throw new ApiError("You are not signed in.", 401);
  const { token } = (await res.json()) as { token: string };
  // The server issues 15 minutes; refresh at 13 so a slow request never
  // straddles the boundary.
  cached = { token, expiresAt: Date.now() + 13 * 60_000 };
  return token;
}

async function getToken(): Promise<string> {
  if (cached && Date.now() < cached.expiresAt) return cached.token;
  // Deduplicate: a page that fires four requests on mount should mint once.
  inFlight ??= mintToken().finally(() => { inFlight = null; });
  return inFlight;
}

export class ApiError extends Error {
  constructor(message: string, readonly status: number, readonly code?: string) {
    super(message);
    this.name = "ApiError";
  }
  get needsConsent() { return this.code === "consent_required"; }
  get rateLimited() { return this.status === 429; }
  get handleTaken() { return this.code === "handle_taken"; }
}

export async function api<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const token = await getToken();
  const res = await fetch(`${BASE}/api/v1${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...(init.headers ?? {}),
    },
  });

  if (res.status === 401 && retry) {
    cached = null;              // stale token, mint a fresh one and try again
    return api<T>(path, init, false);
  }
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string; message?: string };
    throw new ApiError(
      body.message ?? body.error ?? `Request failed (${res.status})`,
      res.status,
      body.error,
    );
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

/**
 * Fired as the tab closes so the summariser job runs.
 *
 * navigator.sendBeacon cannot set an Authorization header, and putting a JWT
 * in a query string leaks it into every access log between here and the
 * server. fetch({keepalive: true}) keeps the request alive through page
 * teardown *and* keeps the credential in a header.
 */
export async function endSessionOnUnload(sessionId: string): Promise<void> {
  const token = cached?.token ?? (await getToken().catch(() => null));
  if (!token) return;
  void fetch(`${BASE}/api/v1/sessions/${sessionId}/end`, {
    method: "POST",
    keepalive: true,
    headers: { Authorization: `Bearer ${token}` },
  }).catch(() => { /* best effort; a missing summary is not worth an error */ });
}

// ---------------------------------------------------------------- API types
// These mirror the Go handlers' JSON exactly. When a handler changes, this
// block is the first thing that should change with it.

export type Me = {
  id: string;
  email: string;
  displayName: string;
  consented: boolean;
  consentVersion: string;
  /** The anonymous name. Empty string means onboarding is not finished. */
  handle: string;
  peerOptIn: boolean;
};

export type Peer = {
  id: string;
  handle: string;
  online: boolean;
  lastSeen: string;
  unread: number;
};

export type PeerMessage = {
  id: string;
  mine: boolean;
  body: string;
  createdAt: string;
  readAt: string | null;
};

export type Note = {
  id: string;
  title: string;
  body: string;
  updatedAt: string;
};

export type SessionSummary = {
  id: string;
  title: string;
  lastActivityAt: string;
};

export type Source = {
  id: string;
  title: string;
  url: string;
  year: number | null;
  score: number;
};

export type StoredMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  createdAt: string;
};

export type SendResult = {
  reply: string;
  crisis: boolean;
  recommendationId?: string;
  sources: Source[];
};
