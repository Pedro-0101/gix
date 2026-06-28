#!/usr/bin/env node
// Fails if any source file exceeds the complex-file ceiling (300 lines).
// Mirrors the `max-lines` ESLint rule but covers both the frontend and the Go
// backend in one cross-platform pass. Data-only files are exempt.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, sep } from "node:path";

const repoRoot = join(import.meta.dirname, "..", "..");
const MAX = 300;
const SOFT = 100; // "simple file" guideline — reported as a hint, not a failure.

// Directories never scanned.
const SKIP_DIRS = new Set([
  "node_modules",
  "dist",
  "bindings",
  ".git",
  "build",
]);

// Files exempt from the ceiling (data-only).
const EXEMPT = new Set([
  join("frontend", "src", "i18n.ts"),
  join("internal", "config", "config.go"),
]);

const EXTS = [".ts", ".tsx", ".go"];

// Test files grow legitimately with table-driven cases — kept off the hard
// ceiling, still surfaced as a hint.
const isTest = (name) =>
  name.endsWith("_test.go") ||
  name.endsWith(".test.ts") ||
  name.endsWith(".test.tsx");

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (!SKIP_DIRS.has(name)) walk(full, out);
    } else if (EXTS.some((e) => name.endsWith(e)) && !name.endsWith(".d.ts")) {
      out.push(full);
    }
  }
  return out;
}

const offenders = [];
const hints = [];
for (const file of walk(repoRoot)) {
  const rel = relative(repoRoot, file);
  if (EXEMPT.has(rel)) continue;
  const lines = readFileSync(file, "utf8").split("\n").length;
  if (lines > MAX && !isTest(file)) offenders.push({ rel, lines });
  else if (lines > SOFT) hints.push({ rel, lines });
}

const norm = (p) => p.split(sep).join("/");
if (hints.length) {
  hints.sort((a, b) => b.lines - a.lines);
  console.log(`Over the ${SOFT}-line "simple" guideline (not failing):`);
  for (const { rel, lines } of hints) console.log(`  ${lines}\t${norm(rel)}`);
  console.log("");
}

if (offenders.length) {
  offenders.sort((a, b) => b.lines - a.lines);
  console.error(`Files over the ${MAX}-line ceiling:`);
  for (const { rel, lines } of offenders) {
    console.error(`  ${lines}\t${norm(rel)}`);
  }
  console.error("\nSplit by responsibility — see the cleanup roadmap in AGENT.md.");
  process.exit(1);
}

console.log(`All scanned files are within the ${MAX}-line ceiling.`);
