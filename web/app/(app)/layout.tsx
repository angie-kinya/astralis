import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import AppGate from "@/components/AppGate";

// A cheap first check: no session cookie, no app. This only reads whether the
// cookie exists — it cannot tell a valid session from an expired one, and does
// not try to. The Go API is the real boundary: an invalid cookie gets a 401 on
// the first request, the API clears it, and the client moves to /signin.
export default async function AppLayout({ children }: { children: React.ReactNode }) {
  if (!cookies().get("astralis_session")) redirect("/signin");
  return <AppGate>{children}</AppGate>;
}
