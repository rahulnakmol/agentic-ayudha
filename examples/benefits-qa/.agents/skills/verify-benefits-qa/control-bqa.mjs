#!/usr/bin/env node
// control-bqa: the control CLI of the verify-benefits-qa skill. PROTOTYPE for
// docs/research/verification-skills-2026-10.md; lives on scratch/verify-skill-prototype only.
// Every command prints JSON on stdout. Exit 0: done; 1: a check failed (drift, a failed journey, an unmet
// criterion); 2: usage or environment error (the message says what to do).
import { execFileSync, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { existsSync, mkdirSync, readdirSync, readFileSync, renameSync, rmSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { homedir, tmpdir } from "node:os";

const skillDir = dirname(fileURLToPath(import.meta.url));
const featuresDir = join(skillDir, "features");
const root = execFileSync("git", ["rev-parse", "--show-toplevel"], { cwd: skillDir, encoding: "utf8" }).trim();
const head = () => execFileSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8" }).trim();

const HELP = `control-bqa: drive benefits-qa for verification (JSON on stdout)

  up [--port N]                 start the app with deploy/serve.sh on port N (default 8791) and wait until ready
  doctor [--port N]             is this instance worth driving? (pid on /healthz is ours, built from HEAD)
  list                          features and journeys from features/*.md
  journey <feature>.<journey>   run one journey; artifacts in .aayudha/verify/<run>/<feature>.<journey>/
  sweep                         run every journey, in the README's order
  evidence --story N --criteria FILE
                                run the journeys .aayudha/evidence/N.toml maps to each criterion and write
                                evidence.json (authority: preview; only the merge gate's pack counts)
  drift                         deterministic source pass: index, sources changed since verified_at,
                                handles no longer in source, routes not in the map
  down [--port N]               stop the instance up started (never by name); evidence stays

  Common flags: --port N, --run ID (artifact directory name; default a timestamp), --dry-run (up and down only:
  say what would happen). Environment: CHROME (browser binary; default agent-browser's or Playwright's).`;

const args = process.argv.slice(2);
const flag = (name, def) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : def;
};
const has = (name) => args.includes(`--${name}`);
const port = Number(flag("port", "8791"));
const base = `http://127.0.0.1:${port}`;
const runDir = join(process.env.TMPDIR || tmpdir(), `benefits-qa-${port}`);
const out = (v, code = 0) => {
  process.stdout.write(JSON.stringify(v, null, 2) + "\n");
  process.exit(code);
};
const die = (msg) => {
  process.stderr.write(`control-bqa: ${msg}\n`);
  process.exit(2);
};
const sha256 = (p) => createHash("sha256").update(readFileSync(p)).digest("hex");

// ---- feature map ----------------------------------------------------------------------------------------------

function blocks(md, tag) {
  const re = new RegExp("```" + tag + "\\n([\\s\\S]*?)\\n```", "g");
  return [...md.matchAll(re)].map((m) => JSON.parse(m[1]));
}

function loadMap() {
  const index = readFileSync(join(featuresDir, "README.md"), "utf8");
  const order = [...index.matchAll(/\]\(([\w-]+)\.md\)/g)].map((m) => m[1]);
  const files = readdirSync(featuresDir).filter((f) => f.endsWith(".md") && f !== "README.md").map((f) => f.slice(0, -3));
  const features = files.map((f) => {
    const md = readFileSync(join(featuresDir, `${f}.md`), "utf8");
    const [feature] = blocks(md, "feature");
    const h2 = [...md.matchAll(/^## (.+)$/gm)].map((m) => m[1]);
    return { file: f, md, ...feature, journeys: blocks(md, "journey"), h2 };
  });
  features.sort((a, b) => order.indexOf(a.file) - order.indexOf(b.file));
  return { order, files, features };
}

function findJourney(ref) {
  const [fid, jid] = ref.split(".");
  const f = loadMap().features.find((x) => x.id === fid);
  const j = f?.journeys.find((x) => x.id === jid);
  if (!j) die(`no journey "${ref}"; run \`control-bqa list\` for the IDs`);
  return { f, j };
}

// ---- instance -------------------------------------------------------------------------------------------------

async function healthPid() {
  try {
    const r = await fetch(`${base}/healthz`, { signal: AbortSignal.timeout(2000) });
    return r.ok ? r.headers.get("x-benefits-qa-pid") : null;
  } catch {
    return null;
  }
}

async function doctor() {
  const state = existsSync(join(runDir, "control.json")) ? JSON.parse(readFileSync(join(runDir, "control.json"), "utf8")) : null;
  const pid = await healthPid();
  const checks = [
    { check: "answers", ok: pid !== null, detail: pid ? `${base}/healthz answered` : `nothing answers ${base}/healthz; run \`control-bqa up --port ${port}\`` },
    { check: "ours", ok: !!state && pid === String(state.pid), detail: state ? `pid on /healthz ${pid}, started ${state.pid}` : "this port was not started by control-bqa up; refusing to drive another instance" },
    { check: "fresh", ok: !!state && state.head === head() && !state.dirty, detail: state ? (state.head === head() ? (state.dirty ? "built from a dirty tree" : "built from HEAD") : `STALE BUILD: built from ${state.head.slice(0, 7)}, HEAD is ${head().slice(0, 7)}; run up again`) : "no build record" },
  ];
  return { ok: checks.every((c) => c.ok), base, checks };
}

// ---- journeys -------------------------------------------------------------------------------------------------

function playwright() {
  for (const pkg of [join(root, "packages/floors/package.json"), join(root, "package.json"), import.meta.url]) { // the last honours NODE_PATH
    try {
      return createRequire(pkg)("playwright");
    } catch {}
  }
  die("Playwright is not installed; run `pnpm install` in packages/floors (or the product's own Playwright)");
}

function chromePath() {
  if (process.env.CHROME) return process.env.CHROME;
  const dir = join(homedir(), ".agent-browser/browsers");
  if (existsSync(dir)) for (const d of readdirSync(dir)) if (existsSync(join(dir, d, "chrome"))) return join(dir, d, "chrome");
  return undefined;
}

function subset(want, got) {
  return Object.entries(want).every(([k, v]) => (typeof v === "object" && v ? subset(v, got?.[k]) : got?.[k] === v));
}

async function runJourney(f, j, run) {
  const dir = join(root, ".aayudha/verify", run, `${f.id}.${j.id}`);
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir, { recursive: true });
  const started = Date.now();
  const actions = [];
  const apiTrace = [];
  const artifacts = [];
  const result = { feature: f.id, journey: j.id, evidence: j.evidence, head: head(), url: base, status: "passed", authority: "preview" };
  const finish = () => {
    for (const name of readdirSync(dir)) {
      if (name === "result.json") continue;
      const p = join(dir, name);
      const kind = name.endsWith(".png") ? "screenshot" : name.endsWith(".webm") ? "recording" : name === "api-trace.jsonl" ? "api-trace" : "report";
      artifacts.push({ path: relative(root, p), kind, sha256: sha256(p), size: statSync(p).size });
    }
    Object.assign(result, { ms: Date.now() - started, artifacts });
    writeFileSync(join(dir, "result.json"), JSON.stringify(result, null, 2) + "\n");
    return result;
  };
  if (j.requires === "events") {
    Object.assign(result, { status: "unreachable", reason: "the [verify] seed launch (deploy/serve.sh) starts the app without -events; see the feature's Gotchas" });
    return finish();
  }
  const doc = await doctor();
  if (!doc.ok) {
    Object.assign(result, { status: "failed", reason: "doctor failed before the drive", doctor: doc.checks });
    return finish();
  }
  const needsPage = j.steps.some((s) => !s.api);
  let browser, ctx, page;
  if (needsPage) {
    const { chromium } = playwright();
    browser = await chromium.launch({ executablePath: chromePath() });
    ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, ...(j.evidence === "recording" ? { recordVideo: { dir, size: { width: 1280, height: 800 } } } : {}) });
    page = await ctx.newPage();
  }
  const step = async (s) => {
    const T = { timeout: 5000 };
    if (s.goto) {
      const r = await page.goto(base + s.goto, { waitUntil: "load" });
      return `HTTP ${r.status()} ${page.url()}`;
    }
    if (s.expect) {
      await page.getByRole(s.expect.role, { name: s.expect.name, exact: true }).first().waitFor({ state: "visible", ...T });
      return `${s.expect.role} "${s.expect.name}" visible`;
    }
    if (s.fill) {
      await page.getByLabel(s.fill.label, { exact: true }).fill(s.fill.value, T);
      return `"${s.fill.label}" = "${s.fill.value}"`;
    }
    if (s.click) {
      await page.getByRole(s.click.role, { name: s.click.name, exact: true }).click(T);
      await page.waitForLoadState("load");
      return `clicked ${s.click.role} "${s.click.name}"; now at ${page.url()}`;
    }
    if (s.expectText) {
      await page.getByText(s.expectText).first().waitFor({ state: "visible", ...T });
      return `text "${s.expectText}" visible`;
    }
    if (s.absentText) {
      const n = await page.getByText(s.absentText).count();
      if (n) throw new Error(`text "${s.absentText}" is present ${n} time(s)`);
      return `text "${s.absentText}" absent`;
    }
    if (s.count) {
      const n = await page.getByRole("region", { name: s.count.within, exact: true }).getByRole(s.count.role).count();
      if (n !== s.count.equals) throw new Error(`${n} ${s.count.role}(s) in "${s.count.within}", want ${s.count.equals}`);
      return `${n} ${s.count.role}(s) in "${s.count.within}"`;
    }
    if (s.screenshot) {
      await page.screenshot({ path: join(dir, s.screenshot), fullPage: true });
      return `screenshot ${s.screenshot}`;
    }
    if (s.api) {
      const a = s.api;
      const init = { method: a.method || "POST", headers: {} };
      if (a.json !== undefined || a.raw !== undefined) {
        init.headers["content-type"] = a.contentType || "application/json";
        init.body = a.raw ?? JSON.stringify(a.json);
      }
      const r = await fetch(base + a.path, init);
      const text = await r.text();
      let body;
      try {
        body = JSON.parse(text);
      } catch {
        body = text;
      }
      apiTrace.push({ request: { method: init.method, path: a.path, contentType: init.headers["content-type"], body: init.body }, response: { status: r.status, body } });
      writeFileSync(join(dir, "api-trace.jsonl"), apiTrace.map((x) => JSON.stringify(x)).join("\n") + "\n");
      if (a.status && r.status !== a.status) throw new Error(`${init.method} ${a.path}: status ${r.status}, want ${a.status}`);
      if (a.body && !subset(a.body, body)) throw new Error(`${init.method} ${a.path}: body ${JSON.stringify(body)} lacks ${JSON.stringify(a.body)}`);
      return `${init.method} ${a.path} → ${r.status}`;
    }
    throw new Error(`unknown step ${JSON.stringify(s)}`);
  };
  try {
    for (const [i, s] of j.steps.entries()) {
      try {
        actions.push({ i, step: s, ok: true, observed: await step(s) });
      } catch (e) {
        actions.push({ i, step: s, ok: false, observed: String(e.message).split("\n")[0] });
        Object.assign(result, { status: "failed", reason: `step ${i}: ${String(e.message).split("\n")[0]}` });
        if (page) await page.screenshot({ path: join(dir, "failure.png"), fullPage: true });
        break;
      }
    }
    if (page) writeFileSync(join(dir, "aria.txt"), await page.locator("body").ariaSnapshot());
  } finally {
    writeFileSync(join(dir, "actions.jsonl"), actions.map((a) => JSON.stringify(a)).join("\n") + "\n");
    if (ctx) {
      const video = page.video();
      await ctx.close();
      if (video) renameSync(await video.path(), join(dir, "recording.webm"));
    }
    if (browser) await browser.close();
  }
  return finish();
}

// ---- evidence ---------------------------------------------------------------------------------------------------

function evidenceMap(story) {
  // The subset of .aayudha/evidence/<story>.toml this prototype reads: [[criterion]] id = N, journeys = [...].
  const p = join(root, ".aayudha/evidence", `${story}.toml`);
  if (!existsSync(p)) die(`${relative(root, p)} does not exist; map each criterion to journeys there`);
  return readFileSync(p, "utf8").split("[[criterion]]").slice(1).map((s) => ({
    id: Number(/id\s*=\s*(\d+)/.exec(s)?.[1]),
    journeys: JSON.parse(/journeys\s*=\s*(\[[^\]]*\])/.exec(s)?.[1] ?? "[]"),
  }));
}

function criteria(file) {
  // Completion Goal lines: N. [kind] state: …; expected: …
  return readFileSync(file, "utf8").split("\n").map((l) => /^(\d+)\. \[([\w-]+)\] state: (.*); expected: (.*)$/.exec(l.trim())).filter(Boolean)
    .map((m) => ({ id: Number(m[1]), kind: m[2], state: m[3], expected: m[4] }));
}

// ---- drift ------------------------------------------------------------------------------------------------------

function drift() {
  const { order, files, features } = loadMap();
  const H2 = ["Sub-features", "How to get to it (user POV)", "Driving it with control-bqa", "Gotchas"];
  const report = { head: head(), index: { missing: files.filter((f) => !order.includes(f)), dead: order.filter((f) => !files.includes(f)) }, features: [] };
  const mapped = new Set(features.flatMap((f) => f.routes || []));
  const appDir = join(root, "examples/benefits-qa");
  const goFiles = execFileSync("git", ["ls-files", "--cached", "--others", "--exclude-standard", "*.go"], { cwd: appDir, encoding: "utf8" }).split("\n").filter((f) => f && !f.endsWith("_test.go"));
  const routes = new Map();
  for (const f of goFiles) {
    for (const m of readFileSync(join(appDir, f), "utf8").matchAll(/HandleFunc\("([^"]+)"/g)) routes.set(m[1], `examples/benefits-qa/${f}`);
  }
  report.unmapped_routes = [...routes].filter(([r]) => !mapped.has(r)).map(([route, file]) => ({ route, file }));
  report.gone_routes = [...mapped].filter((r) => !routes.has(r));
  for (const f of features) {
    const missingSources = f.sources.filter((s) => !existsSync(join(root, s)));
    const text = f.sources.filter((s) => existsSync(join(root, s))).map((s) => readFileSync(join(root, s), "utf8")).join("\n");
    const handles = new Set();
    for (const j of f.journeys) for (const s of j.steps) {
      if (s.click) handles.add(s.click.name);
      if (s.expect) handles.add(s.expect.name);
      if (s.fill) handles.add(s.fill.label);
      if (s.count) handles.add(s.count.within);
      if (s.api) handles.add(s.api.path);
    }
    const found = (h) => [`>${h}<`, `"${h}"`, `'${h}'`, `\`${h}\``].some((q) => text.includes(q)) || [...routes.keys()].some((r) => r.endsWith(" " + h) || r === h);
    const changed = execFileSync("git", ["diff", "--name-only", f.verified_at, "--", ...f.sources], { cwd: root, encoding: "utf8" }).split("\n").filter(Boolean);
    const untracked = execFileSync("git", ["ls-files", "--others", "--exclude-standard", "--", ...f.sources], { cwd: root, encoding: "utf8" }).split("\n").filter(Boolean);
    report.features.push({
      id: f.id,
      contract: H2.every((h, i) => f.h2[i]?.startsWith(h.split(" with")[0])) ? "ok" : `H2s are ${JSON.stringify(f.h2)}`,
      missing_sources: missingSources,
      missing_handles: [...handles].filter((h) => !found(h)),
      stale: [...changed, ...untracked],
      journeys: f.journeys.length,
    });
  }
  const dirty = report.index.missing.length + report.index.dead.length + report.unmapped_routes.length + report.gone_routes.length +
    report.features.reduce((n, f) => n + f.missing_sources.length + f.missing_handles.length + (f.contract === "ok" ? 0 : 1), 0);
  const stale = report.features.filter((f) => f.stale.length).map((f) => f.id);
  report.verdict = dirty ? "drift" : stale.length ? "stale" : "clean";
  report.next = dirty ? "fix the map (doc drift) or the harness, re-drive the affected journeys live, one PR" : stale.length ? `re-drive live: control-bqa sweep (features ${stale.join(", ")} changed since verified_at)` : "nothing to do";
  return report;
}

// ---- commands ---------------------------------------------------------------------------------------------------

const cmd = args[0];
const run = flag("run", new Date().toISOString().replace(/[:.]/g, "-"));
if (!cmd || cmd === "--help" || cmd === "help") {
  process.stdout.write(HELP + "\n");
  process.exit(0);
}
if (cmd === "up") {
  const serve = join(root, "examples/benefits-qa/deploy/serve.sh");
  if (has("dry-run")) out({ would: `BQA_PORT=${port} ${relative(root, serve)}`, runDir });
  const r = spawnSync(serve, { env: { ...process.env, BQA_PORT: String(port) }, encoding: "utf8" });
  if (r.status !== 0) die(`deploy/serve.sh failed:\n${r.stderr || r.stdout}`);
  const dirty = execFileSync("git", ["status", "--porcelain", "--", "examples/benefits-qa", ":!examples/benefits-qa/.agents"], { cwd: root, encoding: "utf8" }).trim() !== "";
  const pid = Number(readFileSync(join(runDir, "pid"), "utf8"));
  writeFileSync(join(runDir, "control.json"), JSON.stringify({ pid, port, head: head(), dirty }) + "\n");
  out({ up: base, pid, head: head(), dirty, log: join(runDir, "log") });
}
if (cmd === "doctor") {
  const d = await doctor();
  out(d, d.ok ? 0 : 1);
}
if (cmd === "list") {
  out(loadMap().features.map((f) => ({ feature: f.id, layers: f.layers, journeys: f.journeys.map((j) => `${f.id}.${j.id} (${j.evidence}${j.requires ? `, requires ${j.requires}` : ""})`) })));
}
if (cmd === "journey") {
  const { f, j } = findJourney(args[1] || "");
  const r = await runJourney(f, j, run);
  out(r, r.status === "passed" ? 0 : 1);
}
if (cmd === "sweep") {
  const results = [];
  for (const f of loadMap().features) for (const j of f.journeys) {
    const r = await runJourney(f, j, run);
    results.push({ journey: `${f.id}.${j.id}`, status: r.status, ms: r.ms, reason: r.reason });
  }
  const failed = results.filter((r) => r.status === "failed").length;
  out({ run, head: head(), passed: results.filter((r) => r.status === "passed").length, failed, unreachable: results.filter((r) => r.status === "unreachable").length, results }, failed ? 1 : 0);
}
if (cmd === "evidence") {
  const story = flag("story");
  const file = flag("criteria");
  if (!story || !file) die("evidence needs --story N and --criteria FILE (the Completion Goal, one criterion per line)");
  const goal = criteria(file);
  const map = evidenceMap(story);
  const report = { story: Number(story), head: head(), authority: "preview", note: "a preview: aayu ship accepts only the merge gate's pack for the PR head", criteria: [], unmet: [] };
  for (const c of goal) {
    const m = map.find((x) => x.id === c.id);
    if (!m || !m.journeys.length) {
      report.unmet.push({ id: c.id, reason: "no journey mapped in the evidence map" });
      continue;
    }
    const entry = { id: c.id, kind: c.kind, journeys: m.journeys, artifacts: [] };
    for (const ref of m.journeys) {
      const { f, j } = findJourney(ref);
      const r = await runJourney(f, j, `${run}-story-${story}`);
      if (r.status !== "passed") {
        report.unmet.push({ id: c.id, reason: `${ref}: ${r.status}${r.reason ? ` (${r.reason})` : ""}` });
        entry.failed = true;
        continue;
      }
      const match = r.artifacts.filter((a) => a.kind === c.kind);
      if (!match.length) {
        report.unmet.push({ id: c.id, reason: `${ref} produced no ${c.kind} (journey evidence: ${j.evidence})` });
        entry.failed = true;
      }
      entry.artifacts.push(...match.map((a) => ({ ...a, criteria: [c.id] })), ...r.artifacts.filter((a) => a.kind === "report").map((a) => ({ ...a, criteria: [c.id] })));
    }
    if (!entry.failed) report.criteria.push(entry);
  }
  const p = join(root, ".aayudha/verify", `${run}-story-${story}`, "evidence.json");
  mkdirSync(dirname(p), { recursive: true });
  writeFileSync(p, JSON.stringify(report, null, 2) + "\n");
  out({ ...report, written: relative(root, p) }, report.unmet.length ? 1 : 0);
}
if (cmd === "drift") {
  const r = drift();
  out(r, r.verdict === "clean" ? 0 : 1);
}
if (cmd === "down") {
  if (!existsSync(join(runDir, "control.json"))) die(`no instance started by control-bqa on port ${port}`);
  const { pid } = JSON.parse(readFileSync(join(runDir, "control.json"), "utf8"));
  if (has("dry-run")) out({ would: `stop pid ${pid} and remove ${runDir}; keep .aayudha/verify/` });
  if ((await healthPid()) === String(pid)) process.kill(pid);
  for (let i = 0; i < 50; i++) {
    try {
      process.kill(pid, 0);
      await new Promise((r) => setTimeout(r, 100));
    } catch {
      break;
    }
  }
  rmSync(runDir, { recursive: true, force: true });
  const kept = existsSync(join(root, ".aayudha/verify")) ? readdirSync(join(root, ".aayudha/verify")).length : 0;
  out({ down: base, stopped: pid, evidence_runs_kept: kept });
}
die(`unknown command "${cmd}"; run control-bqa --help`);
