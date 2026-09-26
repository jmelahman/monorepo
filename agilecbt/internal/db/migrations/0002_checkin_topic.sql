-- What a conversation is about. '' is a daily standup; 'roadmap' is a
-- planning chat from the Roadmap page (kind 'adhoc', no readings).
ALTER TABLE checkins ADD COLUMN topic TEXT NOT NULL DEFAULT '';
