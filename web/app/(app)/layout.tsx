import { redirect } from "next/navigation";

import AppGate from "@/components/AppGate";
import { auth } from "@/auth";

// Server-side guard: an unauthenticated visitor never renders a protected
// screen and then bounces. The Go API enforces the same rule independently —
// this is convenience, the API is the boundary.
export default async function AppLayout({ children }: { children: React.ReactNode }) {
  const session = await auth();
  if (!session?.user) redirect("/");
  return <AppGate>{children}</AppGate>;
}
