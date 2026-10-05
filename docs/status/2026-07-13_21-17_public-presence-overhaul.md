# Status Report — Public Presence Overhaul

**Date:** 2026-07-13 21:17
**Session scope:** README.md improvement, public wiki website creation, GitHub metadata

---

## a) FULLY DONE

1. **GitHub repo metadata set** — description, homepage URL (`https://auditlog.lars.software`), and 16 topics (audit-log, azure-go-workflow, dag, dag-visualization, event-stream, observability, workflow-engine, mermaid, ndjson, html-dashboard, etc.)
2. **README.md improved** — added prominent Documentation/API/Demo links at top, added "Why?" section explaining the observability gap, added 3-step code snippet under the description, updated coverage badge to ~94%, added error classification to features list
3. **Website created from scratch** — full Astro 7 + Starlight + Tailwind v4 site modeled on go-atomic-write/gogenfilter website pattern:
   - **Config files:** package.json, astro.config.mjs, tsconfig.json, firebase.json, .firebaserc, flake.nix, .gitignore, .node-version, .htmlvalidate.json, content.config.ts
   - **Design system:** violet accent (#8b5cf6 — distinct from go-atomic-write emerald and gogenfilter cyan), dark/light theme, DAG-node logo, global.css with full token system, starlight.css override
   - **Public assets:** favicon.svg (DAG node graph), manifest.json, robots.txt, 4 JS files (theme-init, animations, header, copy-code)
   - **Landing page:** HeroSection (syntax-highlighted code, GitHub stars fetch, 4 metrics), FeatureGrid (6 features), HowItWorksSection (3-step Attach/Do/Snapshot), ComparisonSection (9-feature matrix), UseCasesSection (4 use cases), CTASection, Header, Footer, Sections orchestrator
   - **Data layer:** config.ts, types.ts, features.ts, hero-code.ts, sections.ts
   - **10 doc pages:** installation, quick-start, api-reference, event-stream, export-formats, filtering-and-diffing, html-dashboard, changelog, contributing, related-tools
   - **Build verified:** 12 pages generated, Pagefind search index built, sitemap created, zero errors

---

## b) PARTIALLY DONE

1. ~~**Firebase hosting target** — `.firebaserc` specifies target `"auditlog"` on project `"lars-software"`, matching the sibling pattern. However, **the Firebase hosting site `auditlog` does not exist yet** in the Firebase console — it must be created manually or via `firebase hosting:sites:create auditlog`~~ done — hosting site created — 2026-07-13_21-42
2. ~~**DNS / domain** — `auditlog.lars.software` is referenced everywhere (README, robots.txt, sitemap, manifest, config) but **the DNS record and Firebase custom domain connection are not configured**. The site cannot actually be reached at that URL yet~~ done — DNS live — 2026-07-13_21-42
3. ~~**Website git tracking** — `website/` is untracked. Not committed. The entire directory is new and unstaged~~ done — website committed

---

## c) NOT STARTED

1. ~~**Website CI/CD** — no GitHub Actions workflow for building and deploying the website on push. Sibling repos (go-atomic-write, gogenfilter) also lack this, so deployment is currently manual (`nix run .#deploy`)~~ done — website.yml shipped
2. **OG image generation** — gogenfilter has `src/pages/og/[...slug].ts` using astro-og-canvas for social media preview images. Not implemented for this website **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
3. ~~**HTML validation** — `.htmlvalidate.json` config exists but no pnpm script or CI step runs `html-validate` on the built output~~ done — html-validate wired in website CI
4. ~~**`.node-version` git tracking** — file created but website is not committed~~ **Won't implement — .node-version tracked with website/.**
5. **Website typecheck** — `pnpm run typecheck` (astro check) exists in package.json but was never run during this session **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
6. **Website preview/lighthouse** — no Lighthouse audit was run on the built output for performance/accessibility/SEO scores **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
7. **Link checking** — no automated check that all internal links in the website resolve correctly **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
8. **Dependents page** — gogenfilter has a `dependents.astro` page showing projects that use the library. Not applicable yet for this alpha library but could be added later **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**

---

## d) TOTALLY FUCKED UP

Nothing. No errors, no broken builds, no data loss. The website builds cleanly on first try with 12 pages and zero warnings.

---

## e) WHAT WE SHOULD IMPROVE

### Immediate issues I noticed

1. **Website is not committed to git** — the entire `website/` directory is untracked. It needs to be committed and pushed for anyone to see it
2. **The `auditlog.lars.software` domain doesn't resolve** — README and website both link to it, but there is no DNS record and no Firebase hosting site. Users clicking the link get a 404/error. This is a **broken link in the public README right now**
3. **No website deployment workflow** — even if committed, there is no automated path from push to live site. Deployment requires manual `nix run .#deploy` or `pnpm run build && firebase deploy`
4. **Website `package-lock.json` committed but `node_modules/` in `.gitignore`** — this is correct, but the lockfile was generated with pnpm and the `.gitignore` mentions "CI uses pnpm" — need to ensure CI also uses pnpm not bun
5. **README still 527 lines** — the improvement added a "Why?" section and better header, but the README is still extremely long. Some content (detailed API tables, error classification code examples) could be trimmed with "see docs website" links now that the website exists
6. **Coverage badge says ~94%** — the actual coverage gate in CI checks `>=92%`. The AGENTS.md says ~94%. The old README said 93.2%. The badge now says ~94%. These should all be consistent and ideally dynamically linked (e.g. via Codecov)

### Design/content improvements

7. **No screenshots or GIFs** — the landing page is text + code only. A screenshot of the HTML dashboard or an animated GIF of the interactive DAG graph would dramatically improve the "show don't tell" factor
8. **Hero code snippet doesn't show output** — it shows the integration code but not what you get. A small "output preview" panel showing the report summary or dashboard thumbnail would be more compelling
9. **Comparison matrix is onesided** — every row is "no, no, yes" for the library. Adding a row where manual logging has an advantage (e.g. "Zero dependencies") would make it more honest
10. **No "How It Works" diagram** — the 3-step section is good but a visual diagram of the Attach → Do → Snapshot flow with the callback injection mechanism would be more intuitive
11. **Changelog on website is abridged** — I summarized 5 versions. The real CHANGELOG.md has much more detail. The website version could be a direct embed or symlink
12. **No error classification guide page** — the README has a detailed error classification section with code examples and a family table, but the website has no dedicated guide page for this feature
13. **Missing guide: Retry & Timeout tracking** — the library captures retry/timeout config per step, but no guide page explains how to read and use this data
14. **Missing guide: Concurrency model** — the README has a concurrency model section but the website doesn't document it for consumers who need thread-safety guarantees

### Infrastructure improvements

15. **No sitemap submission** — robots.txt points to the sitemap but it is not submitted to Google Search Console
16. **No analytics** — sibling sites likely have no analytics either, but for a public OSS project, basic privacy-respecting analytics (Plausible/Umami) would help understand traffic
17. **No social preview image** — GitHub link unfurls will have no preview image. An OG image (static or generated) would improve social sharing
18. **The `example/` directory** — the README links to `./example` as "Interactive Demo" but this is a Go program, not a web demo. Consider a hosted live demo or at least a screenshot

---

## f) Up to 50 Things to Get Done Next

### Critical (blocks public launch)

1. ~~**Create Firebase hosting site `auditlog`** in Firebase console~~ done — site created — 21-42
2. ~~**Configure DNS** for `auditlog.lars.software` → Firebase hosting~~ done — DNS configured — 21-42
3. ~~**Commit `website/` to git** and push to GitHub~~ done — website committed
4. ~~**Deploy the website** via `firebase deploy --only hosting`~~ done — deployed — 21-42
5. ~~**Verify `auditlog.lars.software` loads** in a browser after DNS propagates~~ done — go-workflow-auditlog.lars.software live
6. **Run `pnpm run typecheck`** (astro check) and fix any TypeScript errors **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
7. ~~**Run html-validate** on `dist/` output and fix any HTML validation issues~~ done — html-validate in CI

### Website improvements

8. **Add OG image generation** (`src/pages/og/[...slug].ts` with astro-og-canvas) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
9. ~~**Add a screenshot of the HTML dashboard** to the landing page~~ done — screenshots shipped (README + docs/screenshots)
10. **Add an animated GIF or video** of the interactive DAG graph in the dashboard **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
11. **Add a "How It Works" visual diagram** (Attach → Do → Snapshot flow) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
12. **Add an error classification guide page** (`/guides/error-classification/`) **→ open — website guide pages (TODO_LIST 2026-10-05)**
13. **Add a retry & timeout tracking guide page** (`/guides/retry-and-timeout/`) **→ open — website guide pages (TODO_LIST 2026-10-05)**
14. **Add a concurrency model guide page** (`/guides/concurrency-model/`) **→ open — website guide pages (TODO_LIST 2026-10-05)**
15. **Add a replay guide page** (`/guides/replay-and-load/`) **→ open — website guide pages (TODO_LIST 2026-10-05)**
16. **Sync website changelog** to import from CHANGELOG.md instead of maintaining a copy **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
17. **Add a "Known Limitations" doc page** based on the README section **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
18. **Add live GitHub star count** to the landing page hero (currently fetched but could add last commit date, contributor count) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
19. **Add a comparison row where manual logging wins** (e.g. "Zero dependencies: yes/yes/no") **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
20. **Add Lighthouse CI** to check performance/accessibility/SEO scores **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
21. **Add link checking** (lychee or similar) to catch broken internal/external links **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
22. **Add a 404 page** design matching the landing page theme (currently uses Starlight default) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
23. **Add a `dependents.astro` page** once the library has external users **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
24. **Add dark/light theme persistence indicator** (visual feedback on current theme) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
25. **Add keyboard navigation** for the comparison matrix table **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
26. **Add code copy buttons** to all doc page code blocks (Starlight has this built-in, verify it works) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**

### README improvements

27. ~~**Trim README** — move detailed API tables and error classification examples to the docs website, replace with links~~ **Won't implement — README deliberately comprehensive — pkg.go.dev landing page.**
28. ~~**Add a "Documentation" callout box** near the top pointing to `auditlog.lars.software`~~ done — Documentation links at top
29. ~~**Add a dashboard screenshot** to the README (after the Example Output section)~~ done — screenshot present
30. ~~**Add a "Comparison with alternatives" section** (like the website comparison matrix)~~ **Won't implement — comparison matrix lives on the website.**
31. **Update coverage badge** to be dynamic (Codecov or similar) instead of hardcoded **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
32. ~~**Add a "Used by" section** once there are external adopters~~ **Won't implement — no external adopters yet.**
33. **Add badges for latest release version** (GitHub Releases badge) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**

### CI/CD

34. ~~**Add website build + deploy workflow** to `.github/workflows/website.yml`~~ done — website.yml shipped
35. ~~**Add website build check** to PR CI (ensure website doesn't break on changes)~~ done — website build check
36. ~~**Add automatic Firebase deploy** on push to master (after DNS is configured)~~ done — auto-deploy on push
37. ~~**Add html-validate to CI** for the website build output~~ done — html-validate in CI
38. **Add lighthouse CI** as a non-blocking status check **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**

### GitHub repo

39. ~~**Add a GitHub social preview image** (1200x630px) in repo settings~~ done — social preview added (3426a4c)
40. ~~**Create a GitHub Discussion** category for Q&A (separate from Issues)~~ **Won't implement — Discussions declined — Issues suffice.**
41. ~~**Set up GitHub Pages** as a fallback/redirect to Firebase hosting~~ **Won't implement — Firebase hosting is canonical.**
42. **Add `website` as a GitHub topic** to make the docs site discoverable **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
43. ~~**Pin an issue** with getting started links and docs URL~~ **Won't implement — pinned issue declined.**

### Content & SEO

44. **Submit sitemap to Google Search Console** after DNS resolves **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
45. **Add structured data** (JSON-LD) for the documentation articles (currently only on landing page) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
46. **Add canonical URLs** to all doc pages **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
47. **Add a blog/changelog feed** (RSS) for release announcements **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
48. **Add meta descriptions** to every doc page frontmatter (some have them, verify all) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
49. **Add Open Graph tags** to doc pages (Starlight adds some, verify they include the site URL) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**
50. **Add a "Edit this page on GitHub" link** to all doc pages (Starlight built-in, verify configured) **→ open — website polish backlog (ROADMAP raw ideas, 2026-10-05)**

---

## g) Top 2 Questions

### 1. ~~Has the Firebase hosting site `auditlog` been created in the `lars-software` project?~~ **Resolved:** yes — site created and deployed in the 2026-07-13_21-42 session (final domain: go-workflow-auditlog.lars.software).

The `.firebaserc` references `"auditlog"` as a hosting target in `"lars-software"`, matching the exact pattern of go-atomic-write (`"atomicwrite"`) and gogenfilter (`"gogenfilter"`). However, Firebase hosting sites must be **created explicitly** before first deploy — `firebase deploy` will fail with "hosting site not found" if the site doesn't exist. I cannot run `firebase hosting:sites:create` because I don't have Firebase CLI credentials in this environment. **Can you run `firebase hosting:sites:create auditlog` (from the `lars-software` project), or has this already been done?**

### 2. ~~Is the DNS record for `auditlog.lars.software` already configured?~~ **Resolved:** domain renamed to go-workflow-auditlog.lars.software; DNS + Firebase custom domain live (2026-07-13_21-42).

The README and entire website assume this URL is live. If DNS is not pointed at Firebase yet, the "Documentation" link in the README is a **broken public link**. I see the sibling pattern uses `<name>.lars.software` subdomains, so I assume you have a wildcard DNS or manual setup process. **Should I change the README link to a placeholder until DNS is confirmed, or is this domain already routing / will be configured imminently?**
