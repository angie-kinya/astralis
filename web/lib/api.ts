// The only place in the frontend that knows the Go API exists.
//
// There are no tokens here any more. Sign-in sets an HttpOnly cookie that
// JavaScript cannot read — by design — and the browser attaches it to every
// same-origin request automatically. The client's whole job is to call
// relative /api/v1 paths and react when the answer is "signed out".

export class ApiError extends Error {
  constructor(message: string, readonly status: number, readonly code?: string) {
    super(message);
    this.name = "ApiError";
  }
  get signedOut()    { return this.status === 401; }
  get needsConsent() { return this.code === "consent_required"; }
  get rateLimited()  { return this.status === 429; }
  get handleTaken()  { return this.code === "handle_taken"; }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    ...init,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", ...(init.headers ?? {}) },
  });

  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string; message?: string };
    const err = new ApiError(
      // Handlers send a human sentence in `message`, or — via fail() — in
      // `error`. Only fall back to a bare status when the body had neither.
      res.status === 401
        ? "Your session has ended. Sign in again."   // body.error is only a code here
        : body.message ?? body.error ?? `Request failed (${res.status})`,
      res.status,
      body.error,
    );
    // Any 401 anywhere means the session is gone (expired, signed out on
    // another device, or the account was deleted). Send the person to sign-in
    // rather than leaving every screen to render its own confusing error.
    if (err.signedOut && typeof window !== "undefined" && !path.startsWith("/auth/")) {
      window.location.assign("/signin");
    }
    throw err;
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function signOut(): Promise<void> {
  await fetch("/api/v1/auth/signout", { method: "POST", credentials: "same-origin" }).catch(() => {});
  window.location.assign("/");
}

/**
 * Fired as the tab closes so a summary can be queued. keepalive lets the
 * request outlive the page; the cookie rides along like any other request.
 * Best effort — cmd/summarise catches anything this misses.
 */
export function endSessionOnUnload(sessionId: string): void {
  void fetch(`/api/v1/sessions/${sessionId}/end`, {
    method: "POST", keepalive: true, credentials: "same-origin",
  }).catch(() => {});
}

// ---------------------------------------------------------------- API types
// These mirror the Go handlers' JSON exactly. When a handler changes, this
// block is the first thing that should change with it.

export type Me = {
  id: string;
  /** Visible to its owner only. No endpoint returns another user's email. */
  email: string;
  consented: boolean;
  consentVersion: string;
  /** The anonymous name. Empty string means onboarding is not finished. */
  handle: string;
  peerOptIn: boolean;
  notifyReleases: boolean;
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
