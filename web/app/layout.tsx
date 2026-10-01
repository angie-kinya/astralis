import type { Metadata, Viewport } from "next";
import { JetBrains_Mono, Sora } from "next/font/google";

import Butterflies from "@/components/Butterflies";

import "./globals.css";

// Self-hosted at build time: no runtime request to Google, nobody outside this
// deployment learns who is reading what.
const mono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono", display: "swap" });
const sans = Sora({ subsets: ["latin"], variable: "--font-sans", display: "swap" });

export const metadata: Metadata = {
  title: "Astralis",
  description: "A research-grounded companion for thinking about work stress and burnout.",
  robots: { index: false, follow: false },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: dark)",  color: "#08070E" },
    { media: "(prefers-color-scheme: light)", color: "#FAF9FC" },
  ],
};

// Runs before first paint so the chosen theme never flashes the wrong one.
const THEME_BOOTSTRAP = `
(function(){try{
  var t = localStorage.getItem("astralis-theme");
  if(!t) t = matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
  document.documentElement.dataset.theme = t;
  document.documentElement.style.colorScheme = t;
}catch(e){document.documentElement.dataset.theme="dark";}})();
`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en-GB" data-theme="dark" suppressHydrationWarning
          className={`${mono.variable} ${sans.variable}`}>
      <head><script dangerouslySetInnerHTML={{ __html: THEME_BOOTSTRAP }} /></head>
      <body className="min-h-dvh font-sans">
        {/* Once, here: behind the landing, sign-in and every module alike. */}
        <Butterflies />
        {children}
      </body>
    </html>
  );
}
