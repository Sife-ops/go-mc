CREATE TABLE seed (
    id INTEGER PRIMARY KEY,
    seed TEXT NOT NULL UNIQUE,
    ravine_chunks INTEGER NOT NULL,
    iron_shipwrecks INTEGER NOT NULL,
    avg_bastion_air INTEGER NOT NULL,
    avg_fortress_air INTEGER NOT NULL,
    played INTEGER DEFAULT 0 NOT NULL,
    rating INTEGER,
    notes TEXT,
    timestamp TEXT DEFAULT CURRENT_TIMESTAMP NOT NULL
);