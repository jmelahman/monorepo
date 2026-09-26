-- llm_session held the Claude Code backend's session id for resuming a
-- check-in; that backend was removed.
ALTER TABLE checkins DROP COLUMN llm_session;
