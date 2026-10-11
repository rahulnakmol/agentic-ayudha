// Written by `aayu install` (WP3.2); `aayu install --apply` rewrites it. Do not edit.
// Amp project plugin: runs the aayudha host hooks. Hooks call only `aayu hook <name>`.
import type { PluginAPI } from '@ampcode/plugin'

const AAYU = "C:\\Program Files\\aayudha\\aayu.exe"

export default function (amp: PluginAPI) {
	amp.on('session.start', async (_event, ctx) => {
		const r = await ctx.$`${AAYU} hook session-start --host amp`
		if (r.stdout.trim() !== '') {
			try {
				await ctx.ui.notify(r.stdout.trim())
			} catch {
				ctx.logger.log(r.stdout.trim()) // no interactive UI, e.g. a headless executor
			}
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
