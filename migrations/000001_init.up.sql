CREATE TABLE stations (
    id          TEXT PRIMARY KEY,
    name_kanji  TEXT NOT NULL,
    name_kana   TEXT,
    name_en     TEXT,
    lat         DOUBLE PRECISION NOT NULL,
    lon         DOUBLE PRECISION NOT NULL,
    altitude_m  INTEGER,
    elems       TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE observations (
    station_id           TEXT        NOT NULL REFERENCES stations (id),
    observed_at          TIMESTAMPTZ NOT NULL,
    temp_c                DOUBLE PRECISION,
    humidity_pct          DOUBLE PRECISION,
    pressure_hpa          DOUBLE PRECISION,
    wind_ms               DOUBLE PRECISION,
    wind_direction_deg    DOUBLE PRECISION,
    precipitation_10m_mm  DOUBLE PRECISION,
    precipitation_1h_mm   DOUBLE PRECISION,
    precipitation_3h_mm   DOUBLE PRECISION,
    precipitation_24h_mm  DOUBLE PRECISION,
    sun_10m_min           DOUBLE PRECISION,
    sun_1h_h              DOUBLE PRECISION,
    snow_cm               DOUBLE PRECISION,
    visibility_m          DOUBLE PRECISION,
    PRIMARY KEY (station_id, observed_at)
);

-- Serves the retention DELETE; the composite PK already serves history queries.
CREATE INDEX observations_observed_at_idx ON observations (observed_at);
