# Status Report — Public Presence: Domain Fix + Firebase Hosting

**Date:** 2026-07-13 21:42
**Session scope:** README/website/domain rename from `auditlog.lars.software` to `go-workflow-auditlog.lars.software`, Firebase hosting site creation, custom domain setup, website deployment

---

## a) FULLY DONE

1. **Firebase hosting site `auditlog` created** in `lars-software` project — `https://auditlog.web.app`
2. **Website deployed** to Firebase hosting — live and verified at `https://auditlog.web.app` (12 pages, full content rendering)
3. **Custom domain `go-workflow-auditlog.lars.software` created** on Firebase via REST API — status: `OWNERSHIP_MISSING` (waiting for DNS), cert: `CERT_VALIDATING`
4. **Old wrong custom domain `auditlog.lars.software` deleted** from Firebase (was created in prior session before the rename)
5. **Terraform DNS records staged** in `domains/lars.software.tf`:
   - `CNAME go-workflow-auditlog → auditlog.web.app.`
   - `TXT _acme-challenge.go-workflow-auditlog → Kl50WF6Xp532l4EmJ2TigjWE2b4u9Njl_-JtkiXmPig` (SSL cert verification)
6. **All website config updated** to `go-workflow-auditlog.lars.software`: `config.ts`, `astro.config.mjs`, `robots.txt`
7. **README link updated** to `https://go-workflow-auditlog.lars.software`
8. **GitHub repo homepage updated** to `https://go-workflow-auditlog.lars.software`
9. **Website rebuilt and redeployed** with corrected domain in all generated URLs/sitemap
10. **Terraform formatted and validated** — config is syntactically correct, ready to apply
11. **Prior session work committed** — the auditlog repo has commit `66e2dc8` (website launch) and the domains repo has commit `7f96f35` (CNAME/TXT records)

---

## b) PARTIALLY DONE

1. **DNS propagation** — Terraform config is staged and validated but **not applied** (blocked by expired Namecheap API key). Firebase shows `OWNERSHIP_MISSING` because the CNAME and TXT records are not live yet. Once DNS propagates, Firebase will auto-verify ownership and provision SSL.
2. **Custom domain SSL** — Firebase cert is in `CERT_VALIDATING` state, waiting for the ACME TXT record to propagate. This will auto-complete once DNS is applied.
3. **Uncommitted domain rename** — 4 files changed in the auditlog repo (`README.md`, `astro.config.mjs`, `robots.txt`, `config.ts`) and 1 file in the domains repo (`lars.software.tf`) have the rename from `auditlog.lars.software` → `go-workflow-auditlog.lars.software` but are not yet committed.

---

## c) NOT STARTED

1. **Website CI/CD** — no GitHub Actions workflow for automatic website build + deploy on push. Deployment is currently manual.
2. **Website typecheck** — `npm run typecheck` (astro check) was never run this session or the prior one.
3. **HTML validation** — `.htmlvalidate.json` exists but was never run against the build output.
4. **OG image generation** — no social media preview images.
5. **Screenshots/GIFs** — landing page is text + code only, no visual previews of the HTML dashboard.
6. **Google Search Console** — sitemap not submitted (domain doesn't resolve yet).

---

## d) TOTALLY FUCKED UP

1. **Prior commit message lies** — commit `66e2dc8` says "launch public documentation website at **auditlog.lars.software**" but the site was never live at that URL (DNS was never applied), and the domain was wrong from the start (should have been `go-workflow-auditlog.lars.software`). The commit message is now stale and misleading in git history.
2. **Prior status report has broken info** — `docs/status/2026-07-13_21-17_public-presence-overhaul.md` references `auditlog.lars.software` throughout and says the domain "needs to be configured" — it has since been deleted and replaced with `go-workflow-auditlog.lars.software`. The report is stale.
3. **Firebase site ID is `auditlog` not `go-workflow-auditlog`** — the hosting site was created as `auditlog` (matching `atomicwrite` and `gogenfilter` short-name pattern). The site ID cannot be renamed. The `.firebaserc` target is `auditlog`. The CNAME points to `auditlog.web.app`. This is correct for the hosting site ID, but could be confusing since the custom domain is `go-workflow-auditlog.lars.software`.

---

## e) WHAT WE SHOULD IMPROVE

### Immediate issues

1. **Commit the domain rename** — 4 files in auditlog repo + 1 file in domains repo are uncommitted. These need to be committed to preserve the rename.
2. **Namecheap API key is expired** — this is the sole blocker for DNS propagation. The key in `domains/terraform.tfvars` returns error 1011102. Without applying Terraform, `go-workflow-auditlog.lars.software` will never resolve.
3. **Amend or fix the stale commit message** — commit `66e2dc8` says "auditlog.lars.software" which is wrong. Could amend (if not pushed) or add a follow-up commit with correct naming.
4. **The prior status report is stale** — references the wrong domain throughout. Should be updated or annotated.

### Structural improvements

5. **No website deploy CI** — every website change requires manual `npm run build && firebase deploy`. A GitHub Actions workflow would auto-deploy on push to master.
6. **No website build verification in PRs** — a broken website build could be merged without detection.
7. **Firebase site ID inconsistency** — `auditlog` vs `go-workflow-auditlog` is a minor confusion point. Not worth recreating the site, but worth documenting.
8. **No custom domain for `go-workflow-auditlog` alias** — only `go-workflow-auditlog.lars.software` is set up. The `go-atomic-write` project has both `atomicwrite.lars.software` and `go-atomic-write.lars.software`. Should we add an `auditlog.lars.software` redirect too? Probably not, since the whole point was disambiguation from `samber-do-auditlog`.
9. **go-output CNAME exists in Terraform without a Firebase site** — `lars.software.tf` has a CNAME for `go-output → go-output.web.app.` but no `go-output` hosting site exists in the `lars-software` project. This is unrelated to our work but was noticed during the investigation. Pre-existing issue.

---

## f) Up to 50 Things to Get Done Next

### Critical (blocks go-live)

1. **Refresh the Namecheap API key** in `domains/terraform.tfvars`
2. **Apply Terraform**: `cd ~/projects/domains && terraform plan -target=namecheap_domain_records.lars_software && terraform apply -target=namecheap_domain_records.lars_software`
3. **Verify DNS propagation**: `dig go-workflow-auditlog.lars.software CNAME`
4. **Verify Firebase auto-provisions SSL** (check Firebase Console → Hosting → Custom Domains)
5. **Verify `https://go-workflow-auditlog.lars.software` loads** in browser after DNS + SSL
6. **Commit the domain rename** in both repos (auditlog + domains)

### Website improvements

7. **Add website CI/CD workflow** (`.github/workflows/website.yml`) — build + deploy on push to master
8. **Add website build check** to PR CI (non-blocking)
9. **Run `npm run typecheck`** and fix any TypeScript errors
10. **Run html-validate** on `dist/` output
11. **Add OG image generation** (`src/pages/og/[...slug].ts`)
12. **Add a screenshot of the HTML dashboard** to the landing page
13. **Add an animated GIF** of the interactive DAG graph
14. **Add error classification guide page** (`/guides/error-classification/`)
15. **Add retry & timeout tracking guide** (`/guides/retry-and-timeout/`)
16. **Add concurrency model guide** (`/guides/concurrency-model/`)
17. **Add replay & load guide** (`/guides/replay-and-load/`)
18. **Add known limitations doc page**
19. **Add Lighthouse CI** for performance/accessibility/SEO
20. **Add link checking** (lychee or similar)
21. **Sync website changelog** from `CHANGELOG.md` instead of maintaining a copy
22. **Add "Edit this page on GitHub" links** (Starlight built-in, verify configured)
23. **Trim README** — move detailed API tables to docs website, replace with links

### Git & repo hygiene

24. **Amend commit `66e2dc8`** to say `go-workflow-auditlog.lars.software` instead of `auditlog.lars.software` (if not pushed; if pushed, add a follow-up commit)
25. **Update or annotate the stale status report** at `docs/status/2026-07-13_21-17_public-presence-overhaul.md`
26. **Add `website` topic** to GitHub repo
27. **Add GitHub social preview image** (1200x630px)
28. **Set up GitHub Discussions** for Q&A
29. **Pin a "Getting Started" issue** with docs URL

### Domain & DNS

30. **Clean up go-output CNAME** — Terraform has a CNAME for `go-output` but no Firebase site exists (pre-existing, unrelated)
31. **Submit sitemap to Google Search Console** after DNS resolves
32. **Add `_acme-challenge` TXT record TTL consideration** — ACME challenges are short-lived; once cert is provisioned Firebase may want a different record. Monitor and update.
33. **Consider CAA records** for `lars.software` to explicitly allow Let's Encrypt + Google PKI for Firebase SSL
34. **Add DNSSEC** to `lars.software` for additional security (if not already enabled)

### Content & SEO

35. **Add structured data** (JSON-LD `TechArticle`) to doc pages
36. **Add canonical URLs** to all doc pages
37. **Add meta descriptions** to every doc page frontmatter
38. **Add Open Graph tags** to doc pages (verify Starlight includes site URL)
39. **Add RSS feed** for changelog/release announcements
40. **Add "Last updated" timestamps** to doc pages
41. **Add reading time estimates** to doc pages
42. **Add "Previous/Next" navigation** at the bottom of doc pages (Starlight built-in)

### Performance

43. **Add `compressHTML` verification** — already enabled, verify output is actually compressed
44. **Add font-display: swap** to web fonts (Astro experimental flag)
45. **Add long-cache headers** for `_astro/` hashed assets in `firebase.json`
46. **Run Lighthouse audit** and fix any performance regressions
47. **Inline critical CSS** for above-the-fold landing page content
48. **Lazy-load below-the-fold images** (when screenshots are added)

### Monitoring

49. **Set up uptime monitoring** for `go-workflow-auditlog.lars.software` (BetterUptime already used for `status.lars.software`)
50. **Set up Firebase Hosting alerts** for deploy failures or quota warnings

---

## g) Top 2 Questions

### 1. Should I amend the stale commit message `66e2dc8` ("auditlog.lars.software") or leave it and move forward?

The commit `66e2dc8` in the auditlog repo says "launch public documentation website at **auditlog.lars.software**" which is wrong — the domain was renamed to `go-workflow-auditlog.lars.software` before it ever went live. If this commit hasn't been pushed to GitHub yet, I can amend it cleanly. If it has been pushed, rewriting history on a public repo is risky. **Has this commit been pushed to `origin/master`, and if so, do you want me to force-push the amend or just move forward with correct naming in future commits?**

### 2. When will the Namecheap API key be refreshed?

This is the **sole blocker** for `go-workflow-auditlog.lars.software` going live. The Terraform DNS records (CNAME + ACME TXT) are staged and validated, but `terraform apply` fails with "API Key is invalid or API access has not been enabled (1011102)". Without applying these records, Firebase shows `OWNERSHIP_MISSING` and the custom domain will never resolve. **Do you have a valid Namecheap API key I can put in `terraform.tfvars`, or will you apply the DNS records yourself?**
