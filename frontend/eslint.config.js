import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";

// Flat config (ESLint 9). The file-size ceiling is enforced here via `max-lines`
// (error at 300) and mirrored by `npm run check:lines`. The ≤100-line target
// for simple files is a guideline documented in AGENT.md, not a hard rule.
export default tseslint.config(
  { ignores: ["bindings/**", "dist/**", "node_modules/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["src/**/*.{ts,tsx}"],
    plugins: { "react-hooks": reactHooks },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "max-lines": [
        "error",
        { max: 300, skipBlankLines: true, skipComments: true },
      ],
      // Pragmatic for this codebase; tighten during the cleanup roadmap.
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
    },
  },
  // Data-only file: exempt from the size ceiling.
  { files: ["src/i18n.ts"], rules: { "max-lines": "off" } },
  // Node tooling scripts run outside the browser.
  {
    files: ["scripts/**/*.{js,mjs}"],
    languageOptions: {
      globals: { console: "readonly", process: "readonly" },
    },
  },
);
