// Regenerates the favicons from the one copy of the mark in
// components/Wordmark.tsx. Run after changing the mark:  node scripts/icons.mjs
// Needs ImageMagick (`magick`) for the PNGs; nothing is added to package.json.
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import ts from "typescript";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const cache = join(root, "node_modules/.cache/astralis-icons");
mkdirSync(cache, { recursive: true });

const src = readFileSync(join(root, "components/Wordmark.tsx"), "utf8");
const { outputText } = ts.transpileModule(src, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX },
});
const mod = join(cache, "Wordmark.mjs");
writeFileSync(mod, outputText);
// react/jsx-runtime resolves from the project, not from the cache directory.
const { markSvg } = await import(pathToFileURL(mod).href + `?t=${Date.now()}`);

// The tokens' violet for each theme: --violet (light) and --violet-2 (dark).
const LIGHT = "#6D3FE0", DARK = "#A78BFA", BG = "#08070E";

// Tab icon: follows the browser's colour scheme.
writeFileSync(join(root, "app/icon.svg"), markSvg({
  color: LIGHT,
  style: `@media (prefers-color-scheme: dark){.m{fill:${DARK};stroke:${DARK}}}`,
}) + "\n");

// PNGs cannot adapt, so they carry their own background: the dark theme, as
// the product does. The mark is inset so the iOS corner mask never clips it.
function png(out, size, inset) {
  const pad = (24 * inset) / (1 - 2 * inset);
  const svg = markSvg({ color: DARK })
    .replace(`viewBox="0 0 24 24"`, `viewBox="${-pad} ${-pad} ${24 + 2 * pad} ${24 + 2 * pad}"`)
    .replace(`<g class="m"`, `<rect x="${-pad}" y="${-pad}" width="${24 + 2 * pad}" height="${24 + 2 * pad}" fill="${BG}"/><g class="m"`);
  const tmp = join(cache, `${size}.svg`);
  writeFileSync(tmp, svg);
  execFileSync("magick", ["-background", "none", "-density", String(Math.ceil((size / 24) * 72 * 4)),
    tmp, "-resize", `${size}x${size}`, "-strip", "-depth", "8", join(root, out)]);
}
png("app/apple-icon.png", 180, 0.16);
png("app/icon1.png", 32, 0.06);
console.log("wrote app/icon.svg, app/icon1.png, app/apple-icon.png");
