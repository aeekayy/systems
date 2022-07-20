CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS urls (
    id      uuid DEFAULT uuid_generate_v4 (),
    original_url    varchar NOT NULL, 
    uri             varchar NOT NULL,
    raw_json        jsonb,
    created timestamptz NOT NULL DEFAULT now(),
    updated timestamptz NOT NULL DEFAULT now(), 
    PRIMARY KEY (id)        
);

CREATE INDEX idx_urls_uri on urls(uri);

CREATE TABLE user_requests(
    id      uuid DEFAULT uuid_generate_v4 (),
    uri     varchar NOT NULL,
    request_json        jsonb,
    created timestamptz NOT NULL DEFAULT now(),
    updated timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id)        
);