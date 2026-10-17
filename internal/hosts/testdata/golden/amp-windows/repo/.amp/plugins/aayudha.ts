// Written by `aayu install` (WP3.2, WP3.2b); `aayu install --apply` rewrites it. Do not edit.
// The aayudha Amp plugin: palette commands for the main verbs, journey state at
// session start, and the host hooks. Hooks call only `aayu hook <name>`; every
// command runs the absolute aayu path install checked, and only after this
// binary reports the version install recorded. Human-only verbs (ship,
// release, decide --accept, respond, init, install) are never offered here:
// a person runs them in a terminal.
import type { PluginAPI, PluginCommandContext } from '@ampcode/plugin'

export const description = 'aayudha: the journey from a grilled idea to a measured release, with proof. Palette commands for the main verbs and your next step.'

const AAYU = "C:\\Program Files\\aayudha\\aayu.exe"
const VERSION = "1.2.3"
// hosts/commands/*/*.md, the host-neutral command sources the Claude Code,
// Codex, and Cursor plugins load too, without /amp (home and next are native below).
const SHARED: { name: string; description: string; hint?: string; body: string }[] = []

// The verbs an agent may run, each sent to the thread as a request to use its skill.
const SKILL_COMMANDS: { verb: string; title: string; ask?: string; prompt: (n: string) => string }[] = [
	{ verb: 'grill', title: 'Shape a story with the Product Strategist', prompt: () => 'Use the aayudha grill skill to shape a new story with me, one question at a time.' },
	{ verb: 'ready', title: 'Check a story for Ready', ask: 'Story or bug number', prompt: (n) => `Use the aayudha ready skill: run \`aayu ready ${n} --check\` and tell me what still blocks Ready.` },
	{ verb: 'plan', title: 'Plan a story', ask: 'Story or bug number', prompt: (n) => `Use the aayudha plan skill on #${n}: one checkpoint per layer.` },
	{ verb: 'build', title: 'Build a Ready story', ask: 'Story or bug number', prompt: (n) => `Use the aayudha build skill on #${n}: I picked it from the palette, so start it with \`aayu build ${n} --asked "build #${n} (command palette)"\`, then commit each checkpoint with \`aayu build ${n}\`.` },
	{ verb: 'verify', title: 'Verify the floors', prompt: () => 'Use the aayudha verify skill on the current change and report each floor.' },
	{ verb: 'inspect', title: 'Inspect a pull request', ask: 'Pull request number', prompt: (n) => `Use the aayudha inspect skill on PR #${n}.` },
]

export default function (amp: PluginAPI) {
	let mismatch = ''
	const ready = (async () => {
		const r = await amp.$`${AAYU} version --json`
		try {
			const v = JSON.parse(r.stdout).version
			if (v !== VERSION) mismatch = `aayu at ${AAYU} reports ${v}, but this plugin was installed by ${VERSION}: run \`aayu install --host amp --apply\` again.`
		} catch {
			mismatch = `aayu at ${AAYU} did not run (exit ${r.exitCode}): run \`aayu install --host amp --apply\` again.`
		}
		return mismatch
	})()

	// guard runs a command only when the binary is the one install checked.
	const guard = (run: (ctx: PluginCommandContext) => Promise<void>) => async (ctx: PluginCommandContext) => {
		if (await ready) {
			await ctx.ui.notify('aayudha: ' + mismatch)
			return
		}
		await run(ctx)
	}

	const show = async (ctx: PluginCommandContext, args: string[]) => {
		const r = await ctx.$`${AAYU} ${args}`
		await ctx.ui.notify((r.stdout + r.stderr).trim() || `aayu ${args.join(' ')}: exit ${r.exitCode}`)
	}

	const ask = async (ctx: PluginCommandContext, content: string) => {
		if (!ctx.thread) {
			await ctx.ui.notify('No active thread: send a message first, then run this command again.')
			return
		}
		await ctx.thread.append([{ type: 'user-message', content }])
	}

	amp.registerCommand('home', { title: 'Home: where the work stands', category: 'aayudha', description: 'The aayudha home screen: journey stage, Ready stories, PRs waiting for a person, and the next step.' },
		guard((ctx) => show(ctx, ['--plain'])))
	amp.registerCommand('next', { title: 'Next step', category: 'aayudha', description: 'The single best next step; the agent takes it when it may.' },
		guard(async (ctx) => {
			const r = await ctx.$`${AAYU} next --json`
			let step: { command: string; why: string; human_only: boolean }
			try {
				step = JSON.parse(r.stdout)
			} catch {
				await ctx.ui.notify((r.stdout + r.stderr).trim())
				return
			}
			if (step.human_only) {
				await ctx.ui.notify(`Next, for you to run in a terminal: ${step.command}\n${step.why}`)
				return
			}
			if (await ctx.ui.confirm({ title: `Next: ${step.command}`, message: step.why, confirmButtonText: 'Ask the agent' })) {
				await ask(ctx, `Run the next aayudha step with its skill: \`${step.command}\` (${step.why})`)
			}
		}))
	amp.registerCommand('status', { title: 'Status of every Goal', category: 'aayudha', description: 'aayu status: the rollup per Goal, Initiative, and Feature.' },
		guard((ctx) => show(ctx, ['status', '--plain'])))
	amp.registerCommand('doctor', { title: 'Doctor: check this machine and repo', category: 'aayudha', description: 'aayu doctor: tools, tokens, config, skill copies, and the MCP server registration.' },
		guard((ctx) => show(ctx, ['doctor', '--plain'])))
	for (const c of SKILL_COMMANDS) {
		amp.registerCommand(c.verb, { title: c.title, category: 'aayudha', description: `Ask the agent to run aayu ${c.verb} with its skill.` },
			guard(async (ctx) => {
				let n = ''
				if (c.ask) {
					n = ((await ctx.ui.input({ title: c.title, helpText: c.ask, submitButtonText: 'Ask the agent' })) ?? '').trim().replace(/^#/, '')
					if (!/^[1-9][0-9]*$/.test(n)) return
				}
				await ask(ctx, c.prompt(n))
			}))
	}

	for (const c of SHARED.filter((c) => c.name !== 'home' && c.name !== 'next')) {
		amp.registerCommand(c.name, { title: c.description || c.name, category: 'aayudha', description: c.description },
			guard(async (ctx) => {
				// A command with an argument hint takes the request after it, as
				// Claude Code, Codex, and Cursor append it.
				let args = ''
				if (c.hint) {
					args = ((await ctx.ui.input({ title: c.description || c.name, helpText: c.hint, submitButtonText: 'Ask the agent' })) ?? '').trim()
					if (args === '') return
				}
				await ask(ctx, args ? `${c.body}\n\nARGUMENTS: ${args}` : c.body)
			}))
	}

	amp.on('session.start', async (_event, ctx) => {
		const lines: string[] = []
		if (await ready) lines.push('aayudha: ' + mismatch)
		const r = await ctx.$`${AAYU} hook session-start --host amp`
		if (r.stdout.trim() !== '') lines.push(r.stdout.trim())
		const s = await ctx.$`${AAYU} hook status-line --host amp`
		if (s.exitCode === 0 && s.stdout.trim() !== '') lines.push(s.stdout.trim())
		if (lines.length === 0) return
		try {
			await ctx.ui.notify(lines.join('\n'))
		} catch {
			ctx.logger.log(lines.join('\n')) // no interactive UI, e.g. a headless executor
		}
	})

	amp.on('tool.call', async (_event, ctx) => {
		const r = await ctx.$`${AAYU} hook pre-tool --host amp`
		if (r.exitCode === 2) {
			return { action: 'reject-and-continue', message: r.stderr.trim() }
		}
		return { action: 'allow' }
	})
}
