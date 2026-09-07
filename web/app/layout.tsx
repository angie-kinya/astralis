import type { Metadata, Viewport } from "next";
import { Inter, JetBrains_Mono, Newsreader } from "next/font/google";

import "./globals.css";

// next/font self-hosts these at build time: no runtime request to Google, no
// third party watching who reads what.
const sans = Inter({ subsets: ["latin"], variable: "--font-sans", display: "swap" });
const serif = Newsreader({
  subsets: ["latin"], variable: "--font-serif", display: "swap", style: ["normal", "italic"],
});
const mono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono", display: "swap" });

export const metadata: Metadata = {
  title: "Astralis",
  description: "A research-grounded companion for thinking about work stress and burnout.",
  robots: { index: false, follow: false },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: dark)", color: "#0A0910" },
    { media: "(prefers-color-scheme: light)", color: "#FBFAF8" },
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
          className={`${sans.variable} ${serif.variable} ${mono.variable}`}>
      <head><script dangerouslySetInnerHTML={{ __html: THEME_BOOTSTRAP }} /></head>
      <body className="min-h-dvh">{children}</body>
    </html>
  );
}
