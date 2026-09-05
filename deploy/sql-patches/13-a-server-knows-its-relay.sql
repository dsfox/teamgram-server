-- A server registers itself once with the relay that holds the Apple and
-- Google keys (#167), and keeps what the relay gave it here.
CREATE TABLE IF NOT EXISTS push_relay (
  url varchar(255) NOT NULL,
  server_id varchar(32) NOT NULL,
  relay_key varchar(128) NOT NULL,
  registered_at int NOT NULL,
  PRIMARY KEY (url)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
