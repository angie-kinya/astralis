import SignIn from "@/components/SignIn";
import ThemeToggle from "@/components/ThemeToggle";
import Wordmark from "@/components/Wordmark";

export const metadata = { title: "Sign in · Astralis" };

// Deliberately does NOT redirect when a cookie is present. If a stale cookie
// ever survived, "signin -> chat -> 401 -> signin" would loop forever; letting
// this page always render makes that impossible.
export default function SignInPage() {
  return (
    <div className="flex min-h-dvh flex-col">
      <header className="flex shrink-0 items-center justify-between border-b border-line px-5 py-3">
        <a href="/" className="rounded"><Wordmark /></a>
        <ThemeToggle />
      </header>
      <div className="flex-1">
        <SignIn />
      </div>
    </div>
  );
}
