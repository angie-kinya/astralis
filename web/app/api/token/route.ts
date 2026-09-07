// The bridge between Auth.js and the Go API.
//
// Auth.js issues an *encrypted* (JWE) session cookie. Go cannot read it, and
// teaching Go to decrypt it would couple the two systems permanently. Instead
// the browser calls this route, we verify the session server-side, and mint a
// short-lived *signed* (HS256 JWS) token that Go verifies with a shared secret.
//
// Two consequences worth understanding:
//   - The secret is symmetric. Anyone holding AUTH_BACKEND_SECRET can mint a
//     token for any user, so it lives only in the two servers' environments.
//   - 15-minute expiry bounds the damage if a token leaks from a browser. The
//     client silently re-mints; the user never sees it.
import { SignJWT } from "jose";
import { NextResponse } from "next/server";

import { auth } from "@/auth";

export const dynamic = "force-dynamic"; // never cache a credential

export async function GET() {
  const session = await auth();
  if (!session?.user) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }

  const subject = (session.user as { id?: string }).id ?? session.user.email;
  if (!subject) {
    return NextResponse.json({ error: "no subject" }, { status: 401 });
  }

  const secret = new TextEncoder().encode(process.env.AUTH_BACKEND_SECRET);
  const token = await new SignJWT({
    email: session.user.email ?? "",
    name: session.user.name ?? "",
  })
    .setProtectedHeader({ alg: "HS256" })
    .setSubject(subject)
    .setIssuedAt()
    .setExpirationTime("15m")
    .sign(secret);

  return NextResponse.json({ token }, {
    headers: { "Cache-Control": "no-store" },
  });
}
