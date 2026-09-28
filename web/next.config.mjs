/** @type {import('next').NextConfig} */

// Where the Go API listens during local development. In production this is
// unused: Caddy sends /api/* straight to Go and never through Next, so long
// LLM replies are not subject to Next's proxy timeout.
const API_ORIGIN = process.env.API_ORIGIN ?? "http://localhost:8080";

export default {
  reactStrictMode: true,
  poweredByHeader: false,
  // A self-contained server bundle for the Docker image — no node_modules to
  // copy, a fraction of the size.
  output: "standalone",

  // One origin for the browser in development too. /api/* is forwarded to Go,
  // so the session cookie Go sets is first-party to localhost:3000 and there
  // is no CORS to configure. Production gets the same property from Caddy.
  async rewrites() {
    if (process.env.NODE_ENV === "production") return [];
    return [{ source: "/api/:path*", destination: `${API_ORIGIN}/api/:path*` }];
  },

  async headers() {
    return [{
      source: "/:path*",
      headers: [
        { key: "X-Content-Type-Options", value: "nosniff" },
        { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
        { key: "X-Frame-Options", value: "DENY" },
        // A private beta handling personal disclosures: never embedded,
        // indexed, or archived.
        { key: "X-Robots-Tag", value: "noindex, nofollow, noarchive" },
      ],
    }];
  },
};
