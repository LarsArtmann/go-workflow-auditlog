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

1. **Firebase hosting target** — `.firebaserc` specifies target `"auditlog"` on project `"lars-software"`, matching the sibling pattern. However, **the Firebase hosting site `auditlog` does not exist yet** in the Firebase console — it must be created manually or via `firebase hosting:sites:create auditlog`
2. **DNS / domain** — `auditlog.lars.software` is referenced everywhere (README, robots.txt, sitemap, manifest, config) but **the DNS record and Firebase custom domain connection are not configured**. The site cannot actually be reached at that URL yet
3. **Website git tracking** — `website/` is untracked. Not committed. The entire directory is new and unstaged

---

## c) NOT STARTED

1. **Website CI/CD** — no GitHub Actions workflow for building and deploying the website on push. Sibling repos (go-atomic-write, gogenfilter) also lack this, so deployment is currently manual (`nix run .#deploy`)
2. **OG image generation** — gogenfilter has `src/pages/og/[...slug].ts` using astro-og-canvas for social media preview images. Not implemented for this website
3. **HTML validation** — `.htmlvalidate.json` config exists but no npm script or CI step runs `html-validate` on the built output
4. **`.node-version` git tracking** — file created but website is not committed
5. **Website typecheck** — `npm run typecheck` (astro check) exists in package.json but was never run during this session
6. **Website preview/lighthouse** — no Lighthouse audit was run on the built output for performance/accessibility/SEO scores
7. **Link checking** — no automated check that all internal links in the website resolve correctly
8. **Dependents page** — gogenfilter has a `dependents.astro` page showing projects that use the library. Not applicable yet for this alpha library but could be added later

---

## d) TOTALLY FUCKED UP

Nothing. No errors, no broken builds, no data loss. The website builds cleanly on first try with 12 pages and zero warnings.

---

## e) WHAT WE SHOULD IMPROVE

### Immediate issues I noticed

1. **Website is not committed to git** — the entire `website/` directory is untracked. It needs to be committed and pushed for anyone to see it
2. **The `auditlog.lars.software` domain doesn't resolve** — README and website both link to it, but there is no DNS record and no Firebase hosting site. Users clicking the link get a 404/error. This is a **broken link in the public README right now**
3. **No website deployment workflow** — even if committed, there is no automated path from push to live site. Deployment requires manual `nix run .#deploy` or `npm run build && firebase deploy`
4. **Website `package-lock.json` committed but `node_modules/` in `.gitignore`** — this is correct, but the lockfile was generated with npm and the `.gitignore` mentions "CI uses npm" — need to ensure CI also uses npm not bun
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

1. **Create Firebase hosting site `auditlog`** in Firebase console
2. **Configure DNS** for `auditlog.lars.software` → Firebase hosting
3. **Commit `website/` to git** and push to GitHub
4. **Deploy the website** via `firebase deploy --only hosting`
5. **Verify `auditlog.lars.software` loads** in a browser after DNS propagates
6. **Run `npm run typecheck`** (astro check) and fix any TypeScript errors
7. **Run html-validate** on `dist/` output and fix any HTML validation issues

### Website improvements

8. **Add OG image generation** (`src/pages/og/[...slug].ts` with astro-og-canvas)
9. **Add a screenshot of the HTML dashboard** to the landing page
10. **Add an animated GIF or video** of the interactive DAG graph in the dashboard
11. **Add a "How It Works" visual diagram** (Attach → Do → Snapshot flow)
12. **Add an error classification guide page** (`/guides/error-classification/`)
13. **Add a retry & timeout tracking guide page** (`/guides/retry-and-timeout/`)
14. **Add a concurrency model guide page** (`/guides/concurrency-model/`)
15. **Add a replay guide page** (`/guides/replay-and-load/`)
16. **Sync website changelog** to import from CHANGELOG.md instead of maintaining a copy
17. **Add a "Known Limitations" doc page** based on the README section
18. **Add live GitHub star count** to the landing page hero (currently fetched but could add last commit date, contributor count)
19. **Add a comparison row where manual logging wins** (e.g. "Zero dependencies: yes/yes/no")
20. **Add Lighthouse CI** to check performance/accessibility/SEO scores
21. **Add link checking** (lychee or similar) to catch broken internal/external links
22. **Add a 404 page** design matching the landing page theme (currently uses Starlight default)
23. **Add a `dependents.astro` page** once the library has external users
24. **Add dark/light theme persistence indicator** (visual feedback on current theme)
25. **Add keyboard navigation** for the comparison matrix table
26. **Add code copy buttons** to all doc page code blocks (Starlight has this built-in, verify it works)

### README improvements

27. **Trim README** — move detailed API tables and error classification examples to the docs website, replace with links
28. **Add a "Documentation" callout box** near the top pointing to `auditlog.lars.software`
29. **Add a dashboard screenshot** to the README (after the Example Output section)
30. **Add a "Comparison with alternatives" section** (like the website comparison matrix)
31. **Update coverage badge** to be dynamic (Codecov or similar) instead of hardcoded
32. **Add a "Used by" section** once there are external adopters
33. **Add badges for latest release version** (GitHub Releases badge)

### CI/CD

34. **Add website build + deploy workflow** to `.github/workflows/website.yml`
35. **Add website build check** to PR CI (ensure website doesn't break on changes)
36. **Add automatic Firebase deploy** on push to master (after DNS is configured)
37. **Add html-validate to CI** for the website build output
38. **Add lighthouse CI** as a non-blocking status check

### GitHub repo

39. **Add a GitHub social preview image** (1200x630px) in repo settings
40. **Create a GitHub Discussion** category for Q&A (separate from Issues)
41. **Set up GitHub Pages** as a fallback/redirect to Firebase hosting
42. **Add `website` as a GitHub topic** to make the docs site discoverable
43. **Pin an issue** with getting started links and docs URL

### Content & SEO

44. **Submit sitemap to Google Search Console** after DNS resolves
45. **Add structured data** (JSON-LD) for the documentation articles (currently only on landing page)
46. **Add canonical URLs** to all doc pages
47. **Add a blog/changelog feed** (RSS) for release announcements
48. **Add meta descriptions** to every doc page frontmatter (some have them, verify all)
49. **Add Open Graph tags** to doc pages (Starlight adds some, verify they include the site URL)
50. **Add a "Edit this page on GitHub" link** to all doc pages (Starlight built-in, verify configured)

---

## g) Top 2 Questions

### 1. Has the Firebase hosting site `auditlog` been created in the `lars-software` project?

The `.firebaserc` references `"auditlog"` as a hosting target in `"lars-software"`, matching the exact pattern of go-atomic-write (`"atomicwrite"`) and gogenfilter (`"gogenfilter"`). However, Firebase hosting sites must be **created explicitly** before first deploy — `firebase deploy` will fail with "hosting site not found" if the site doesn't exist. I cannot run `firebase hosting:sites:create` because I don't have Firebase CLI credentials in this environment. **Can you run `firebase hosting:sites:create auditlog` (from the `lars-software` project), or has this already been done?**

### 2. Is the DNS record for `auditlog.lars.software` already configured?

The README and entire website assume this URL is live. If DNS is not pointed at Firebase yet, the "Documentation" link in the README is a **broken public link**. I see the sibling pattern uses `<name>.lars.software` subdomains, so I assume you have a wildcard DNS or manual setup process. **Should I change the README link to a placeholder until DNS is confirmed, or is this domain already routing / will be configured imminently?**
