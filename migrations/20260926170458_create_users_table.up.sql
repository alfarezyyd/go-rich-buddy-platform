CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL,
    tier VARCHAR(50) NOT NULL DEFAULT 'Starter',
    credit_balance INT NOT NULL DEFAULT 5
)