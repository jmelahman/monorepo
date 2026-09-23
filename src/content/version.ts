/**
 * The balance stamp on the golden vectors.
 *
 * Bump this whenever a change alters a number a recorded run could observe: a
 * letter value, a target, a relic's arithmetic, a payout, the answer list, the
 * order anything is drawn in. Then re-record (`npm run golden`) and read the
 * diff: that diff *is* the balance change, stated in scores rather than in
 * source.
 *
 * The vectors refuse to run against a version they were not recorded at, so
 * forgetting to bump this fails loudly instead of quietly rewriting the
 * baseline.
 */
export const CONTENT_VERSION = 23
