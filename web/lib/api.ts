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

/**
 * The only way an error becomes text on screen.
 *
 * ApiError messages were written by the server, for people ("That code is
 * wrong or has expired"), so they are shown as they are. Anything else is
 * either the network or a bug in this code — "Cannot read properties of null"
 * is meaningless and alarming to the person reading it. They get a plain
 * sentence; the real error goes to the console, where a developer can see it.
 */
export function userMessage(e: unknown, fallback = "Something went wrong. Try again in a moment."): string {
  if (e instanceof ApiError) return e.message;
  console.error(e);
  // fetch rejects with a TypeError when the request never reached a server.
  if (e instanceof TypeError && /fetch|network/i.test(e.message)) {
    return "Could not reach Astralis. Check your connection and try again.";
  }
  return fallback;
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
  /** When they accepted the current terms; null if they have not. */
  consentedAt: string | null;
  /** When the terms last changed. Shown on the consent sheet. */
  termsUpdatedAt: string;
  /** The anonymous name. Empty string means onboarding is not finished. */
  handle: string;
  peerOptIn: boolean;
  notifyReleases: boolean;
  /** constellations is behind GAMES_ENABLED; false hides every entry point */
  games: boolean;
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
  /** null once deleted: the row stays as a tombstone so the thread still reads */
  body: string | null;
  deleted: boolean;
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

export type LatestItem = {
  title: string;
  source: string;
  category: "finance" | "tech" | "business";
  publishedAt: string;
  summary: string;
  url: string;
};

/** one row of person_facts: something they mentioned that astralis keeps, visible and deletable in settings */
export type Fact = {
  id: string;
  kind: "person" | "event" | "commitment" | "helped" | "context" | string;
  subject: string;
  detail: string;
  status: "open" | "done" | "dropped" | string;
  mentions: number;
  firstSeen: string;
  lastSeen: string;
};

export type ReportReason = "harassment" | "spam" | "unsafe" | "other";

// ---------------------------------------------------------------- games
// one game model, three engines. everything here is kind-agnostic; the view a
// game carries is shaped per kind (ConstellationsView below, WordsView and
// SudokuView in the puzzles block at the end of this file)

export type GameKind = "constellations" | "words" | "sudoku";
export type GameMode = "peer" | "astralis" | "solo";
/** 0 is nobody, 1 is player_a, 2 is player_b (or astralis, or the second colour in solo) */
export type Player = 0 | 1 | 2;

export type GameOptions = {
  level?: number;
  topic?: string;
  /** words: classic | anagram | mystery */
  mode?: string;
  /** set on daily puzzles, the nairobi day they belong to */
  day?: string;
};

/** options sent when creating a game; daily is a request, day is what comes back */
export type CreateOptions = { level?: number; topic?: string; mode?: string; daily?: boolean };

/**
 * edge numbering: horizontal first, e = r*(cols-1)+c joins (r,c)-(r,c+1);
 * then vertical, e = H + r*cols + c joins (r,c)-(r+1,c), H = rows*(cols-1).
 * box b = r*(cols-1)+c. an edge is playable only if it borders an active box
 */
export type ConstellationsView = {
  /** stars per side */
  rows: number;
  cols: number;
  edges: number;
  /** per box: part of the board (orion is a mask) */
  active: boolean[];
  drawn: boolean[];
  drawnBy: Player[];
  owner: Player[];
  toMove: 1 | 2;
  lastEdge: number | null;
};

/** shaped for whoever asked: me, myTurn, score and result differ between the two players */
export type Game = {
  id: string;
  kind: GameKind;
  mode: GameMode;
  level: number;
  options: GameOptions;
  /** null for astralis and solo */
  peerId: string | null;
  peerHandle: string | null;
  me: 1 | 2;
  /** puzzles and duels: true while active and I am not done */
  myTurn: boolean;
  status: "active" | "finished" | "abandoned";
  result: "won" | "lost" | "draw" | "solved" | "unsolved" | null;
  score: { me: number; them: number } | null;
  /** narrow by kind: `game.view as WordsView` and so on. a constellations game always carries ConstellationsView */
  view: ConstellationsView | { [k: string]: unknown };
  /** constellations vs astralis: edges astralis drew in reply to this request, in order */
  botEdges: number[];
  version: number;
  updatedAt: string;
};

export type Invite = {
  id: string;
  peerId: string;
  peerHandle: string;
  /** true when I sent it */
  mine: boolean;
  status: "pending" | "accepted" | "declined" | "expired";
  /** set once accepted */
  gameId: string | null;
  createdAt: string;
  expiresAt: string;
  kind: "constellations" | "words";
  options: GameOptions;
};

export type Catalog = {
  constellations: { levels: { level: number; name: string; rows: number; cols: number; boxes: number }[] };
  words: {
    topics: { id: string; name: string; blurb: string }[];
    modes: { id: string; name: string; blurb: string }[];
  };
  sudoku: { levels: { level: number; name: string; size: number }[] };
};

/** highest playable constellations level */
export type Progress = { constellations: { unlocked: number } };

export type LeaderRow = { handle: string; points: number; games: number; me: boolean };
export type Leaderboard = {
  top: LeaderRow[];
  me: { points: number; games: number; optedIn: boolean };
};

// ---------------------------------------------------------------- calls
// thin wrappers, so a route change touches one line and callers stay typed

export const listFacts   = () => api<{ facts: Fact[] }>("/me/facts");
export const deleteFact  = (id: string) => api<void>(`/me/facts/${id}`, { method: "DELETE" });
export const lastTime    = () => api<{ fact: Fact | null }>("/me/last-time");
export const rateSession = (id: string, worth: boolean) =>
  api<void>(`/sessions/${id}/rating`, { method: "POST", body: JSON.stringify({ worth }) });

export const deletePeerMessage = (peerId: string, msgId: string) =>
  api<void>(`/peers/${peerId}/messages/${msgId}`, { method: "DELETE" });
export const blockPeer   = (peerId: string) => api<void>(`/peers/${peerId}/block`, { method: "POST" });
export const unblockPeer = (peerId: string) => api<void>(`/peers/${peerId}/block`, { method: "DELETE" });
export const reportPeer  = (peerId: string, reason: ReportReason, messageId?: string) =>
  api<void>(`/peers/${peerId}/report`, { method: "POST", body: JSON.stringify({ reason, messageId }) });

export const sendInvite    = (peerId: string, kind?: Invite["kind"], options?: GameOptions) =>
  api<{ invite: Invite }>(`/peers/${peerId}/invites`, {
    method: "POST", body: JSON.stringify(kind ? { kind, options: options ?? {} } : {}),
  });
export const listInvites   = () => api<{ invites: Invite[] }>("/games/invites");
export const acceptInvite  = (id: string) => api<{ game: Game }>(`/games/invites/${id}/accept`, { method: "POST" });
/** either side; from the sender it is a cancel */
export const declineInvite = (id: string) => api<void>(`/games/invites/${id}/decline`, { method: "POST" });
export const createGame    = (kind: GameKind, mode: "astralis" | "solo", options: CreateOptions = {}) =>
  api<{ game: Game }>("/games", { method: "POST", body: JSON.stringify({ kind, mode, options }) });
export const activeGames   = () => api<{ games: Game[] }>("/games/active");
export const getGame       = (id: string) => api<{ game: Game }>(`/games/${id}`);
/** move is per kind: {edge} | {guess} | {hint} | {grid}. version is the one the move was made against */
export const playMove      = (id: string, move: Record<string, unknown>, version: number) =>
  api<{ game: Game }>(`/games/${id}/moves`, { method: "POST", body: JSON.stringify({ move, version }) });
export const catalog       = () => api<Catalog>("/games/catalog");
export const progress      = () => api<Progress>("/games/progress");
export const leaderboard   = () => api<Leaderboard>("/games/leaderboard");
export const setLeaderboardOptIn = (optIn: boolean) =>
  api<void>("/me/leaderboard-opt-in", { method: "POST", body: JSON.stringify({ optIn }) });

// ---------------------------------------------------------------- puzzles
// the views a words or sudoku game carries in game.view. owned by the puzzle
// components (components/words, components/sudoku); append-only

export type LetterMark = "correct" | "present" | "absent";

export type WordsView = {
  length: number;
  maxGuesses: number;
  mode: "classic" | "anagram" | "mystery";
  /** null in mystery until the puzzle is over */
  topic: string | null;
  /** anagram only */
  scrambled: string | null;
  guesses: { word: string; marks: LetterMark[] }[];
  letters: Record<string, LetterMark>;
  /** revealed so far, in order: clue, a letter, first and last */
  hints: string[];
  hintsLeft: number;
  solved: boolean;
  over: boolean;
  /** answer and fact arrive only once over */
  answer: string | null;
  clue: string | null;
  fact: string | null;
  /** duel only, and never the other person's letters */
  opponent: { guesses: number; solved: boolean; over: boolean } | null;
};

export type SudokuView = {
  size: 6 | 9;
  boxRows: number;
  boxCols: number;
  level: number;
  /** n*n chars, '0' empty */
  givens: string;
  entries: string;
  solved: boolean;
};
