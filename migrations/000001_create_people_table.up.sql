CREATE TABLE people (
    human_id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    surname TEXT,
    patronymic TEXT,
    age INTEGER,
    gender TEXT,
    country TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);