-- Create a table to store shared experiments.
CREATE TABLE IF NOT EXISTS shared_experiments (
  public_id           TEXT  PRIMARY KEY,

  created_at          TIMESTAMPTZ DEFAULT NOW(),
  last_retrieved_at   TIMESTAMPTZ,

  hash TEXT UNIQUE NOT NULL,

  -- IP address of the person sharing the experiment.
  client_ip      INET NOT NULL,

  dynamic_config  TEXT NOT NULL,
  request        JSONB NOT NULL,
  result         JSONB NOT NULL
);
