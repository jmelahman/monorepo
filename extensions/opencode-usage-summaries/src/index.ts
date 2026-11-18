import { Plugin } from "@opencode/plugin"

/**
 * The server part is a no-op: usage data comes from the client's
 * `experimental.session.stats` endpoint, so only the TUI part
 * (`./tui`) has work to do. It exists so the package follows the
 * usual `.` + `./tui` layout and loads automatically from `plugins`.
 */
export default Plugin.define({
  id: "usage-summaries.server",
  async setup() {},
})
