import { redirect } from "next/navigation";

import AppFrame from "@/components/AppFrame";
import { auth } from "@/auth";

// Server-side guard. The client never gets a chance to render a protected
// screen and then bounce; if there is no session, this redirects during SSR.
// The Go API enforces the same rule independently — this is convenience, the
// API is the boundary.
export default async function AppLayout({ children }: { children: React.ReactNode }) {
  const session = await auth();
  if (!session?.user) redirect("/");

  return <AppFrame name={session.user.name ?? session.user.email ?? "you"}>{children}</AppFrame>;
}
