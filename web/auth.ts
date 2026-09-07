// Auth.js owns login and nothing else. It never talks to the Go API, never
// touches the application database, and knows nothing about consent or
// sessions. Its single output is "this browser belongs to GitHub user X".
import NextAuth from "next-auth";
import GitHub from "next-auth/providers/github";

export const { handlers, auth, signIn, signOut } = NextAuth({
  providers: [GitHub],
  session: { strategy: "jwt" },
  callbacks: {
    // Pin the subject to the stable numeric GitHub id. Email and username can
    // both change; if the subject changed with them, the Go side would create
    // a second user row and the person would silently lose their history.
    async jwt({ token, profile }) {
      if (profile?.id) token.sub = String(profile.id);
      return token;
    },
    async session({ session, token }) {
      if (token.sub) (session.user as { id?: string }).id = token.sub;
      return session;
    },
  },
  pages: { signIn: "/" },
});
