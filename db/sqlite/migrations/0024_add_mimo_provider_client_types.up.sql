PRAGMA foreign_keys = OFF;

ALTER TABLE providers RENAME TO providers_old;

CREATE TABLE providers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  client_type TEXT NOT NULL DEFAULT 'openai-completions',
  icon TEXT,
  enable INTEGER NOT NULL DEFAULT 1,
  config TEXT NOT NULL DEFAULT '{}',
  metadata TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT providers_name_unique UNIQUE (name),
  CONSTRAINT providers_client_type_check CHECK (client_type IN (
    'openai-responses',
    'openai-completions',
    'anthropic-messages',
    'google-generative-ai',
    'openai-codex',
    'github-copilot',
    'edge-speech',
    'openai-speech',
    'openai-transcription',
    'openrouter-speech',
    'openrouter-transcription',
    'elevenlabs-speech',
    'elevenlabs-transcription',
    'deepgram-speech',
    'deepgram-transcription',
    'minimax-speech',
    'volcengine-speech',
    'alibabacloud-speech',
    'microsoft-speech',
    'google-speech',
    'google-transcription',
    'mimo-speech',
    'mimo-transcription'
  ))
);

INSERT INTO providers (
  id,
  name,
  client_type,
  icon,
  enable,
  config,
  metadata,
  created_at,
  updated_at
)
SELECT
  id,
  name,
  client_type,
  icon,
  enable,
  config,
  metadata,
  created_at,
  updated_at
FROM providers_old;

DROP TABLE providers_old;

PRAGMA foreign_key_check;
PRAGMA foreign_keys = ON;
